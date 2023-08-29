package main

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
)

var cli struct {
	Debug     bool   `help:"Debug mode."`
	Clean     bool   `help:"Clean files before running."`
	StartFile string `help:"File path to watch for start." short:"s" type:"path" default:"./start"`
	EndFile   string `help:"File path to watch for end." short:"e" type:"path" default:"./end"`
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
	if _, err := os.Stat(cli.StartFile); errors.Is(err, os.ErrNotExist) {
		// file does not exist
		log.Debug("Start file does not exist, this is good")
	} else {
		// file exists
		if !cli.Clean {
			log.Fatal("Start file exists, this is bad")
		}
		log.Debug("Start file exists, cleaning")
		err = os.Remove(cli.StartFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Check for end file
	if _, err := os.Stat(cli.EndFile); errors.Is(err, os.ErrNotExist) {
		// file does not exist
		log.Debug("End file does not exist, this is good")
	} else {
		// file exists
		if !cli.Clean {
			log.Fatal("End file exists, this is bad")
		}
		log.Debug("End file exists, cleaning")
		err = os.Remove(cli.EndFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Create a channel to notify the main goroutine we're done
	done := make(chan struct{})

	// Create a channel for waiting on the transaction to finish
	wait := make(chan struct{})

	// Semaphore for checking that we started before ending
	started := make(chan struct{})

	// Transaction function - this is where we do the work
	// Done as an inline for easy referencing closure variables
	// TODO: Hoist this and pass in channels
	transaction := func() {
		// TODO: Call metrics functions
		log.Info("Action started")

		// Reading the file contents so we can do stuff with the data there
		start, err := ioutil.ReadFile(cli.StartFile)
		if err != nil {
			log.Fatal("Could not read start file", "err", err)
		}
		log.Debug("Start file contents", "contents", string(start))

		// Hang out here until we're finished
		<-wait

		// Same reading end file contents, this may not matter
		end, err := ioutil.ReadFile(cli.EndFile)
		if err != nil {
			log.Fatal("Could not read end file", "err", err)
		}
		log.Debug("End file contents", "contents", string(end))

		log.Info("Action finished")

		// Exit the program
		close(done)
	}

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// TODO: Remove, it's super noisy
				// log.Debug("Event", "event", event.Op)
				if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) {
					// We only care about create and write events to these files
					continue
				}

				log.Debug("Event", "path", event.Name)

				// Handle StartFile modification
				if event.Name == cli.StartFile {
					log.Debug("Start file modified")
					// Handle multiple events which may try to close the start
					// semaphore more than once or launch the transaction
					select {
					case <-started:
						// Already started, do nothing
					default:
						// Close our semaphore channel
						close(started)
						// Launch the transaction async
						go transaction()
					}
					// Wait for more events
					continue
				}

				// Handle EndFile modification
				if event.Name == cli.EndFile {
					log.Debug("End file modified")
					select {
					case <-started:
						log.Debug("Action started, closing")
					default:
						log.Fatal("Action not started")
						continue
					}

					close(wait)
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
	<-done

	// TODO: Any other finishing up outside the transaction can happen here
	log.Debug("Done")
}
