package main_test

import (
	"runtime"
	"sync"
	"testing"

	. "github.com/shakefu/gha-debug"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSoftLock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = Describe("SoftLock", func() {
	var sl *SoftLock = nil
	BeforeEach(func() {
		if sl != nil {
			sl.Close()
			sl = nil
		}
		sl = NewSoftLock()
	})

	Context("Start", func() {
		It("should be true on first call", func() {
			Expect(sl.Start()).To(BeTrue())
		})

		It("should be false on second call", func() {
			Expect(sl.Start()).To(BeTrue())
			Expect(sl.Start()).To(BeFalse())
		})
	})

	Context("Started", func() {
		It("should be false on first call", func() {
			Expect(sl.Started()).To(BeFalse())
		})
		It("should be true after Start() is called", func() {
			Expect(sl.Start()).To(BeTrue())
			Expect(sl.Started()).To(BeTrue())
		})
	})

	Context("Wait", func() {
		It("should block until started", func() {
			var count = 1
			go func() {
				count += 1
				sl.Start()
				count += 1
				go func() {
					count += 1
					sl.Release()
					count += 1
				}()
			}()

			Expect(count).To(Equal(1))
			sl.Wait()
			Expect(count).To(Equal(5))
		})
	})

	Context("Release", func() {
		var m sync.Mutex

		BeforeEach(func() {
			m = sync.Mutex{}
		})

		It("should release a waiting goroutine", func() {
			By("starting the lock")
			sl.Start()
			m.Lock()
			go func() {
				By("release the soft lock")
				sl.Release()
				By("unlocking the mutex")
				m.Unlock()
			}()
			By("checking that we're blocked")
			Expect(m.TryLock()).To(BeFalse())
			By("waiting for the soft lock to be released")
			sl.Wait()
			By("checking that we're unblocked")
			Expect(m.TryLock()).To(BeTrue())
		})

		It("should do nothing if not started", func() {
			sl.Release()
			Expect(sl.Released()).To(BeFalse())
			sl.Start()
			Expect(sl.Released()).To(BeFalse())
		})

		It("should change released state", func() {
			sl.Release()
			Expect(sl.Released()).To(BeFalse())
			sl.Start()
			Expect(sl.Released()).To(BeFalse())
			sl.Release()
			Expect(sl.Released()).To(BeTrue())
		})
	})

	Context("Close", func() {
		var m sync.Mutex

		BeforeEach(func() {
			m = sync.Mutex{}
		})

		It("should clean up the soft lock", func() {
			// Lock the mutex
			m.Lock()
			// Schedule goroutine to unlock when done
			go func() {
				sl.WaitForDone()
				m.Unlock()
			}()
			// Can't acquire the lock
			Expect(m.TryLock()).To(BeFalse())
			// Yield to the goroutine (blocked by WaitForDone)
			runtime.Gosched()
			// Can't acquire the lock
			Expect(m.TryLock()).To(BeFalse())
			// Close also calls Done
			sl.Close()
			// Yield to goroutine to unlock
			runtime.Gosched()
			// Can acquire the lock
			Expect(m.TryLock()).To(BeTrue())
			// All the state has progressed
			Expect(sl.Started()).To(BeTrue())
			Expect(sl.Released()).To(BeTrue())
			Expect(sl.Finished()).To(BeTrue())
		})

		It("should work on a started lock", func() {
			Expect(sl.Started()).To(BeFalse())
			sl.Start()
			Expect(sl.Started()).To(BeTrue())
			sl.Close()
			// All the state has progressed
			Expect(sl.Started()).To(BeTrue())
			Expect(sl.Released()).To(BeTrue())
			Expect(sl.Finished()).To(BeTrue())
		})

		It("should work on a released lock", func() {
			sl.Start()
			Expect(sl.Released()).To(BeFalse())
			sl.Release()
			Expect(sl.Released()).To(BeTrue())
			sl.Close()
			// All the state has progressed
			Expect(sl.Started()).To(BeTrue())
			Expect(sl.Released()).To(BeTrue())
			Expect(sl.Finished()).To(BeTrue())
		})

		It("should work on a done lock", func() {
			sl.Start()
			sl.Release()
			Expect(sl.Finished()).To(BeFalse())
			sl.Done()
			Expect(sl.Finished()).To(BeTrue())
			sl.Close()
			// All the state has progressed
			Expect(sl.Started()).To(BeTrue())
			Expect(sl.Released()).To(BeTrue())
			Expect(sl.Finished()).To(BeTrue())

		})
	})

	Context("WaitForDone", func() {
		var m sync.Mutex

		BeforeEach(func() {
			m = sync.Mutex{}
		})

		It("should block until done", func() {
			// Start the lock
			sl.Start()
			// Schedule a goroutine to release the lock
			go func() {
				sl.Release()
				// Yield back to the closure
				runtime.Gosched()
				sl.Done()
				m.Unlock()
			}()
			// Lock the mutex
			m.Lock()
			// Yield to the release goroutine
			runtime.Gosched()
			// Hasn't been unlocked yet
			Expect(m.TryLock()).To(BeFalse())
			// Wait for the goroutine to finish (yielding)
			sl.WaitForDone()
			// Lock is finished
			Expect(sl.Finished()).To(BeTrue())
			// Mutex is unlocked
			Expect(m.TryLock()).To(BeTrue())
		})
	})
})
