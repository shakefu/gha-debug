package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// SoftLock implements an idepotent two stage locking mechanism based on
// channels to allow for asynchronous triggering of waiting goroutines.
// Once it has been used, it cannot be reused.
type SoftLock struct {
	started chan struct{} // started gives an explicit signal for try-once semantics
	wait    chan struct{} // wait is the main lock
	done    chan struct{} // done is the signal that we're finished, and can exit
	m       sync.Mutex    // m protects the channels from concurrent access
}

// NewSoftLock creates a new SoftLock instance.
func NewSoftLock() *SoftLock {
	return &SoftLock{
		started: make(chan struct{}, 1),
		wait:    make(chan struct{}, 1),
		done:    make(chan struct{}, 1),
	}
}

// Start the lock and return true if we started, false if we were already
// started.
func (l *SoftLock) Start() bool {
	l.m.Lock()
	defer l.m.Unlock()
	select {
	case <-l.started:
		// Already started, do nothing
		return false
	default:
		// Close our semaphore channel
		close(l.started)
		// Launch the transaction async
		return true
	}
}

// Started returns whether or not we've started our transaction.
func (l *SoftLock) Started() bool {
	select {
	case <-l.started:
		// Already started, do nothing
		return true
	default:
		// Close our semaphore channel
		return false
	}
}

// Release the soft lock allowing waiting goroutines to continue.
func (l *SoftLock) Release() {
	l.m.Lock()
	defer l.m.Unlock()
	select {
	case <-l.started:
		// We've started, try to release the wait
		select {
		case <-l.wait:
			// Already released, do nothing
		default:
			// Close our wait signal
			close(l.wait)
		}
	default:
		// Not started, do nothing
	}
}

// Released returns true if the main wait lock has been released
func (l *SoftLock) Released() bool {
	select {
	case <-l.wait:
		// Already released
		return true
	default:
		// Not released
		return false
	}
}

// Wait for the soft lock to be released.
func (l *SoftLock) Wait() {
	// TODO: Decide if this should be a passthrough if the lock was not started
	select {
	case <-l.wait:
		// Already released, do nothing
	default:
		// Wait for the release
		<-l.wait
	}
}

// Done indicates all the soft lock work is finished, and we can exit.
func (l *SoftLock) Done() {
	l.m.Lock()
	defer l.m.Unlock()
	select {
	case <-l.done:
		// Already done, do nothing
	default:
		// Close our done signal
		close(l.done)
	}
}

// Finished returns true if the lock is finished
func (l *SoftLock) Finished() bool {
	select {
	case <-l.done:
		// Already done
		return true
	default:
		// Not done
		return false
	}
}

// Close forces the soft lock to be done, and we can exit.
func (l *SoftLock) Close() {
	l.Start()
	l.Release()
	l.Done()
}

// WaitForDone waits for the soft lock to completely finish its lifecycle.
func (l *SoftLock) WaitForDone() {
	<-l.done
}

// Transaction represents a single transaction to be monitored by NewRelic
type Transaction struct {
	app       *newrelic.Application // NewRelic application instance
	txn       *newrelic.Transaction // NewRelic transaction instance
	lock      *SoftLock             // Our shared lock for the transaction
	startFile string                // Filename to read for starting context
	endFile   string                // Filename to read for ending context
	workflow  string                // Workflow name
	job       string                // Job name
}

// NewTransaction creates a new Transaction instance and initializes the NewRelic app
func NewTransaction(newRelicApp string, newRelicLicense string, lock *SoftLock, startFile string, endFile string) *Transaction {
	// Create new NR app
	var app *newrelic.Application
	var err error

	// Mock out the NR App if we don't have a license
	if newRelicApp == "" || newRelicLicense == "" {
		// This is nil-safe/the correct mocking behavior according to:
		// https://pkg.go.dev/github.com/newrelic/go-agent/v3@v3.24.1/newrelic#Application
		app = nil
		// app = &newrelic.Application{}
	} else {
		appName := newrelic.ConfigAppName(newRelicApp)
		license := newrelic.ConfigLicense(newRelicLicense)
		// Create the NR Application for this transaction
		app, err = newrelic.NewApplication(appName, license)
		if err != nil {
			// This is a hard failure, we can't continue, so we panic
			log.Fatal(err)
		}
	}
	t := &Transaction{
		app:       app,
		txn:       nil,
		lock:      lock,
		startFile: startFile,
		endFile:   endFile,
	}
	return t
}

// Monitor function - this is where we do the work
func (t *Transaction) Monitor() {
	// Ensure we always end nicely
	defer t.Cleanup()

	log.Info("Action started")

	// Parse our context file so we can reference our names correctly
	t.ParseContext(t.startFile)

	// Create the transaction name based on workflow and job
	transaction := fmt.Sprintf("%s / %s", t.workflow, t.job)
	t.txn = t.app.StartTransaction(transaction)

	// Hang out here until we're finished
	t.lock.Wait()

	// Parse the end file
	// TODO: Figure out if there's extra info in here that we actually want
	// TODO: Figure out if we can get success/fail status without calling the API
	t.ParseContext(t.endFile)

	log.Info("Action finished")
}

// ParseContext parses the GitHub Action context file JSON
func (t *Transaction) ParseContext(filename string) {
	var err error
	var data []byte

	data, err = os.ReadFile(filename)
	if err != nil {
		log.Fatal("Could not read context file", "err", err)
	}
	log.Debug("Context file", "context", string(data))

	var context map[string]interface{}
	err = json.Unmarshal(data, &context)
	if err != nil {
		log.Fatal("Could not parse context file", "err", err)
	}

	// TODO: Pull the workflow name and job name from the context
}

// CleanupTransaction gives us a way to reliably end the transaction and clean up
func (t *Transaction) Cleanup() {
	// Ensure the lock is fully released and the program can exit fully, even if
	// something goofy happens with the NR calls below
	defer t.lock.Close()

	// End the NR transaction
	t.txn.End()

	// Default to 60s timeout sending data to NR
	t.app.Shutdown(60 * time.Second)
}

// TODO: Determine if we want to allow GH repo name/Workflow name/Job name to be
// passed in, optionally supplied by Env vars, or parsed from the context JSON
var cli struct {
	Debug           bool   `help:"Debug mode."`
	Clean           bool   `help:"Clean files before running."`
	StartFile       string `help:"File path to watch for start." short:"s" type:"path" default:"./start"`
	EndFile         string `help:"File path to watch for end." short:"e" type:"path" default:"./end"`
	NewRelicLicense string `help:"NewRelic license." short:"l" type:"string" default:"" env:"NEW_RELIC_LICENSE_KEY"`
	Repo            string `help:"GitHub repository." short:"r" type:"string" env:"GITHUB_REPOSITORY"`
	Workflow        string `help:"GitHub workflow." short:"w" type:"string" env:"GITHUB_WORKFLOW"`
	Job             string `help:"GitHub job ID." short:"j" type:"string" env:"GITHUB_JOB"`
	Branch          string `help:"GitHub branch." short:"b" type:"string" env:"GITHUB_HEAD_REF"`
}

func main() {
	// ctx := kong.Parse(&cli,
	_ = kong.Parse(&cli,
		kong.Name("gha-debug"),
		kong.Description("A GitHub Actions debug tool."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
			Summary: true,
		}))

	// TODO: Decide if we want to JSON format logs
	// log.SetFormatter(log.JSONFormatter)

	// Configure the logger for debug output
	if cli.Debug {
		log.SetLevel(log.DebugLevel)
		log.Debug("Debug mode enabled")
	}

	// What files are we watching?
	log.Debug("Args", "--start-file", cli.StartFile, "--end-file", cli.EndFile)

	// Check for files and optionally clean them
	CheckOrClean(cli.StartFile, cli.Clean)
	CheckOrClean(cli.EndFile, cli.Clean)

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Our lock for the transaction
	lock := NewSoftLock()

	// Create a new transaction
	// TODO: Pass in workflow info/figure out if we want to use a shared struct to pass this around
	transaction := NewTransaction(cli.Repo, cli.NewRelicLicense, lock, cli.StartFile, cli.EndFile)

	// Start listening for FS events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) {
					// We only care about create and write events to these files
					continue
				}

				log.Debug("Event", "path", event.Name)

				// Handle StartFile modification
				if event.Name == cli.StartFile {
					log.Debug("Start file modified")
					// Idempotently launch the transaction
					if lock.Start() {
						go transaction.Monitor()
					}
					// Wait for more events
					continue
				}

				// Handle EndFile modification
				if event.Name == cli.EndFile {
					log.Debug("End file modified")

					if !lock.Started() {
						log.Fatal("Action not started")
						continue
					}

					log.Debug("Action started, closing")
					lock.Release()
					return
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("Error", "err", err)
			}
		}
	}()

	// Get our start/end direectories so we can watch for creation of new files
	// (can't watch a file that doesn't exist)
	startPath := filepath.Dir(cli.StartFile)
	endPath := filepath.Dir(cli.EndFile)

	// Add the directories that we're watching for our start/end file flags.
	err = watcher.Add(startPath)
	if err != nil {
		log.Fatal(err)
	}
	if endPath != startPath {
		// Can only add a path once
		err = watcher.Add(endPath)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Wait for notification that we're done - the EndFile was modified.
	lock.WaitForDone()
	log.Debug("Done")
}

// CheckOrClean checks for the existence of the start/end files, and optionally
// cleans them up if they exist
func CheckOrClean(filename string, clean bool) {
	// Check for files and optionally clean them
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		// file does not exist
		log.Debug("Flag file does not exist, this is good")
	} else {
		// file exists
		if !clean {
			log.Fatal("Flag file exists, this is bad", "filename", filename)
		}
		log.Debug("Flag file exists, cleaning", "filename", filename)
		err = os.Remove(filename)
		if err != nil {
			log.Fatal(err)
		}
	}
}
