// Package fileflag provides the FileFlag type, which lets you use asynchronous flags for interprocess semaphore.
package fileflag

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/fsnotify/fsnotify"
	"github.com/shakefu/gha-debug/pkg/softlock"
)

type FileFlag struct {
	filename string
	lock     *softlock.SoftLock
	watcher  *fsnotify.Watcher
}

// NewFileFlag creates a new FileFlag.
func NewFileFlag(filename string) (ff *FileFlag, err error) {
	log.Debug("Creating new FileFlag", "filename", filename)
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
	} else if err != nil {
		// Something else happened
		log.Error("Error", "err", err)
		return
	} else {
		log.Debug("File exists")
		// It exists, start the lock
		ff.lock.Start()
	}

	for {
		select {
		case event, ok := <-ff.watcher.Events:
			log.Debug("Got", "event", event, "ok", ok)
			// If there's nothing on the channel, keep going
			if !ok {
				return
			}

			log.Debug("Name", "name", event.Name)
			// If the event isn't for our file, keep going
			if event.Name != ff.filename {
				continue
			}

			// If the event is our file being created, start the lock
			if event.Has(fsnotify.Create) {
				log.Debug("File created")
				ff.lock.Start()
				continue
			}

			// If the event is our file being removed, release the lock
			if event.Has(fsnotify.Remove) {
				log.Debug("File removed")
				ff.lock.Release()
				continue
			}
		case err, ok := <-ff.watcher.Errors:
			if !ok {
				return
			}
			ff.Close()
			log.Error("Error", "err", err)
		}
	}
}

// WaitForStart blocks until the flag exists. If it already exists, it is a
// passthrough.
func (ff *FileFlag) WaitForStart() {
	// This might not actually be useful but anyway.
	if _, err := os.Stat(ff.filename); errors.Is(err, os.ErrNotExist) {
		// file does not exist
		ff.lock.WaitForStart()
	}
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

// Close closes the FileFlag and disables its watcher. This will also release all waits.
func (ff *FileFlag) Close() {
	defer ff.watcher.Close()
	defer ff.lock.Close()
}
