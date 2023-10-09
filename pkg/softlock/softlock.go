// Package softlock is a multistage asynchronous lock suitable for use in multiprocess sempahore.
package softlock

import (
	"fmt"
	"sync"
)

// SoftLock implements an idepotent two stage locking mechanism based on
// channels to allow for asynchronous triggering of waiting goroutines.
// Once it has been used, it cannot be reused.
type SoftLock struct {
	_started bool // _started is a flag to indicate we've started,
	// which softens the lock further allowing Wait() passthrough without yielding
	// the running goroutine

	started chan struct{} // started gives an explicit signal for try-once semantics
	wait    chan struct{} // wait is the main lock
	done    chan struct{} // done is the signal that we're finished, and can exit
	m       sync.Mutex    // m protects the channels from concurrent access
}

func (l *SoftLock) String() string {
	return fmt.Sprintf("SoftLock(started=%t, released=%t, finished=%t)", l.Started(), l.Released(), l.Finished())
}

// NewSoftLock creates a new SoftLock instance.
func NewSoftLock() *SoftLock {
	return &SoftLock{
		_started: false,
		started:  make(chan struct{}),
		wait:     make(chan struct{}),
		done:     make(chan struct{}),
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
		l._started = true
		return l._started
	}
}

// Started returns whether or not we've started our transaction.
func (l *SoftLock) Started() bool {
	l.m.Lock()
	defer l.m.Unlock()
	return l._started
}

// Release the soft lock allowing waiting goroutines to continue.
func (l *SoftLock) Release() {
	l.m.Lock()
	defer l.m.Unlock()
	if !l._started {
		// If we're not started, we don't release
		return
	}

	// We've started, try to release the wait
	select {
	case <-l.wait:
		// Already released, do nothing
	default:
		// Close our wait signal
		close(l.wait)
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

// Wait for the soft lock to be released. If the lock has not been started, this
// will be a passthrough.
func (l *SoftLock) Wait() {
	l.m.Lock()
	if !l._started {
		defer l.m.Unlock()
		return
	}
	l.m.Unlock()
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

// WaitForDone waits for the soft lock to completely finish its lifecycle. This
// will block regardless of whether the lock has started or not.
func (l *SoftLock) WaitForDone() {
	<-l.done
}

// WaitForStart waits for the soft lock to start. If the lock has already been
// started, this will be a passthrough.
func (l *SoftLock) WaitForStart() {
	l.m.Lock()
	if l._started {
		defer l.m.Unlock()
		return
	}
	l.m.Unlock()
	<-l.started
}
