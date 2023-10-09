// Package fileflag provides the FileFlag type, which lets you use asynchronous flags for interprocess semaphore.
package fileflag

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	_log "github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/shakefu/gha-debug/pkg/softlock"
)

func nullLog(msg interface{}, keyvals ...interface{}) {}

var log = _log.NewWithOptions(os.Stderr, _log.Options{Prefix: "FileFlag"})
var silly = nullLog

func init() {
	if os.Getenv("DEBUG") != "" {
		log.SetLevel(_log.DebugLevel)
	}
	if os.Getenv("SILLY") != "" {
		log.SetLevel(_log.DebugLevel)
		silly = log.Debug
	}
}

type FileFlag struct {
	filename string
	lock     *softlock.SoftLock
	watcher  *fsnotify.Watcher
	m        *sync.Mutex
}

// NewFileFlag creates a new FileFlag.
func NewFileFlag(filename string, m ...*sync.Mutex) (ff *FileFlag, err error) {
	silly("Creating new FileFlag", "filename", filename)

	// Create our watcher first
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	// Can't watch for non-existent files, so we watch directories instead
	path := filepath.Dir(filename)

	// Watch the directory which will contain, eventually, our target file
	err = watcher.Add(path)
	if err != nil {
		return
	}

	// Create a new instance and return it
	ff = &FileFlag{
		filename: filename,
		lock:     softlock.NewSoftLock(),
		watcher:  watcher,
	}

	return
}

// Watch is our goroutine for watching for changes.
func (ff *FileFlag) Watch() {
	// If the file exists, start the lock
	if _, err := os.Stat(ff.filename); errors.Is(err, os.ErrNotExist) {
		// Doesn't exist, we're good
		silly("Flag does not exist")
	} else if err != nil {
		// Something else happened
		log.Error("Error", "err", err)
		return
	} else {
		silly("Flag exists, starting")
		// It exists, start the lock
		ff.lock.Start()
	}

	silly("Watching", "filename", ff.filename)
	for {
		silly("Waiting for events")
		// Explicit yield to the scheduler, so we don't hang?
		// runtime.Gosched()
		select {
		case event, ok := <-ff.watcher.Events:
			silly("Got", "ok", ok, "event", event)
			// If there's nothing on the channel, keep going
			if !ok {
				return
			}

			// If the event isn't for our file, keep going
			if event.Name != ff.filename {
				continue
			}

			// If the event is our file being created, start the lock
			if event.Has(fsnotify.Create) {
				silly("Flag created")
				ff.lock.Start()
				silly("Lock started")
				continue
			}

			// If the event is our file being removed, release the lock
			if event.Has(fsnotify.Remove) {
				silly("Flag removed")
				ff.lock.Release()
				silly("Lock released")
				return
			}
		case err, ok := <-ff.watcher.Errors:
			if !ok {
				log.Error("Watcher error", "err", err)
				return
			}
			defer ff.Close()
			log.Fatal("Error", "err", err)
		case <-time.After(200 * time.Millisecond):
			// This timeout clause is because it seems like fsnotify can
			// drop/lose events, so we need to manually check our lock state
			// otherwise it hangs indefinitely
			log.Warn("Timeout")
			// We've been hanging out in this too long, let's check our lock manually
			_, err := os.Stat(ff.filename)
			silly("Checking flag")
			if err == nil {
				// File exists, start the lock
				silly("Flag exists")
				ff.lock.Start()
				silly("Lock started")
				continue
			} else if os.IsNotExist(err) {
				// File does not exist, release the lock, if it was already started
				silly("Flag does not exist")
				if ff.lock.Started() {
					ff.lock.Release()
					silly("Lock released")
					return
				} else {
					silly("Lock not started")
				}
			} else {
				// Some other error, log it and bail
				log.Error("Error", "err", err)
				return
			}
		}
	}
}

// WaitForStart blocks until the flag exists. If it already exists, it is a
// passthrough.
func (ff *FileFlag) WaitForStart() {
	if ff.lock.Started() {
		return
	}
	ff.lock.WaitForStart()
}

// Wait blocks until the flag has been removed. If the flag is already removed,
// it is a passthrough.
func (ff *FileFlag) Wait() {
	ff.lock.Wait()
}

// WaitForDone blocks until the flag has completely been resolved.
func (ff *FileFlag) WaitForDone() {
	ff.lock.WaitForDone()
}

// Close closes the FileFlag and disables its watcher. This will also release
// all waits. This method is nil-safe.
func (ff *FileFlag) Close() {
	if ff == nil {
		return
	}
	defer ff.watcher.Close()
	defer ff.lock.Close()
}
