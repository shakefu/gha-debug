// Package fileflag provides the FileFlag type, which lets you use asynchronous flags for interprocess semaphore.
package fileflag

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/shakefu/gha-debug/pkg/softlock"
)

type FileFlag struct {
	filename string
	lock     *softlock.SoftLock
	watcher  *fsnotify.Watcher
	watching chan struct{}
}

// NewFileFlag creates a new FileFlag.
func NewFileFlag(filename string) (ff *FileFlag, err error) {
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
		watching: make(chan struct{}),
	}

	return
}

// Watch is our goroutine for watching for changes.
func (ff *FileFlag) Watch() {
	// If the file exists, start the lock
	if _, err := os.Stat(ff.filename); errors.Is(err, os.ErrNotExist) {
		// Doesn't exist, we're good
	} else if err != nil {
		// Something else happened
		log.Error("Error", "err", err)
		return
	} else {
		// It exists, start the lock
		ff.lock.Start()
	}

	// Signal that we've started watching for the file flag
	select {
	case <-ff.watching:
		// Already started, do nothing
	default:
		// Close our semaphore channel
		close(ff.watching)
	}

	for {
		// Explicit yield to the scheduler, so we don't hang?
		// runtime.Gosched()
		select {
		case event, ok := <-ff.watcher.Events:
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
				ff.lock.Start()
				continue
			}

			// If the event is our file being removed, release the lock
			if event.Has(fsnotify.Remove) {
				ff.lock.Release()
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
			// This timeout implements a pollling behavior (yuck), with a 200ms
			// interval as a back-up for the watcher. If there's a long running
			// task, this will be harmlessly invoked manually checking the file,
			// which won't exist
			//
			// This can also happen if the file is created while we're setting
			// up the watcher - the file creation event will be lost, and the
			// lock will never be started. This is a workaround for that.
			if !ff.lock.Started() {
				log.Warn("FileFlag timeout, use FileFlag.WaitForWatch()", "filename", ff.filename)
			}
			// We've been hanging out in this too long, let's check our lock manually
			_, err := os.Stat(ff.filename)
			if err == nil {
				// File exists, start the lock
				ff.lock.Start()
				continue
			} else if os.IsNotExist(err) {
				// File does not exist, release the lock, if it was already started
				if ff.lock.Started() {
					ff.lock.Release()
					return
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
	ff.WaitForWatch()
	if ff.lock.Started() {
		return
	}
	ff.lock.WaitForStart()
}

// Wait blocks until the flag has been removed. If the flag is already removed,
// it is a passthrough.
func (ff *FileFlag) Wait() {
	ff.WaitForStart()
	ff.lock.Wait()
}

// WaitForWatch blocks until the flag has been watched.
func (ff *FileFlag) WaitForWatch() {
	select {
	case <-ff.watching:
		// Already watching, do nothing
	default:
		<-ff.watching
	}
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
	// We wait for watching
	select {
	case <-ff.watching:
		// Already closed, do nothing
	default:
		defer close(ff.watching)
	}
	defer ff.watcher.Close()
	defer ff.lock.Close()
}
