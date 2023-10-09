package softlock_test

import (
	"runtime"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/shakefu/gha-debug/pkg/softlock"
)

func TestSoftLock(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SoftLock Suite")
}

var _ = Describe("SoftLock", func() {
	Context("Simple tests", func() {
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
			It("should be false before we start", func() {
				Expect(sl.Started()).To(BeFalse())
			})
			It("should be true after Start() is called", func() {
				Expect(sl.Start()).To(BeTrue())
				Expect(sl.Started()).To(BeTrue())
			})
		})
	})

	Context("Wait", func() {
		It("should not block until started", func() {
			sl := NewSoftLock()
			done := make(chan interface{})

			By("checking lock is unstarted")
			Expect(sl.Started()).To(BeFalse())
			Expect(sl.Released()).To(BeFalse())
			Expect(sl.Finished()).To(BeFalse())

			// By("creating closure goroutine")
			go func() {
				// By("starting lock")
				sl.Start()
			}()

			go func() {
				// By("releasing lock")
				sl.WaitForStart()
				sl.Release()
			}()

			go func() {
				sl.Wait()
				close(done)
			}()

			// This is actually non-deterministic ... the By() seems to yield

			// By("waiting")
			Eventually(done).Should(BeClosed())

			Expect(sl.Started()).To(BeTrue())
			Expect(sl.Released()).To(BeTrue())
			Expect(sl.Finished()).To(BeFalse())

			sl.Done()
			Expect(sl.Finished()).To(BeTrue())
		})
	})

	Context("Release", func() {
		var m sync.Mutex

		BeforeEach(func() {
			m = sync.Mutex{}
		})

		It("should release a waiting goroutine", func() {
			sl := NewSoftLock()
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
			sl := NewSoftLock()
			sl.Release()
			Expect(sl.Released()).To(BeFalse())
			sl.Start()
			Expect(sl.Released()).To(BeFalse())
		})

		It("should change released state", func() {
			sl := NewSoftLock()
			sl.Release()
			Expect(sl.Released()).To(BeFalse())
			sl.Start()
			Expect(sl.Released()).To(BeFalse())
			sl.Release()
			Expect(sl.Released()).To(BeTrue())
		})
	})

	Context("Close", func() {
		It("should clean up the soft lock", func() {
			done := make(chan interface{})
			sl := NewSoftLock()

			// Schedule goroutine to unlock when done
			go func() {
				sl.WaitForDone()
				close(done)
			}()

			// Close also calls Done
			sl.Close()

			// Wait for the goroutine to finish
			Eventually(done).Should(BeClosed())

			Expect(sl.Started()).To(BeTrue())
			Expect(sl.Released()).To(BeTrue())
			Expect(sl.Finished()).To(BeTrue())
		})

		It("should work on a started lock", func() {
			sl := NewSoftLock()
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
			sl := NewSoftLock()
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
			sl := NewSoftLock()
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
		It("should block until done", func() {
			sl := NewSoftLock()
			done := make(chan interface{})
			// Start the lock
			sl.Start()
			// Schedule a goroutine to release the lock
			go func() {
				sl.Release()
				// Yield back to the closure
				runtime.Gosched()
				sl.Done()
				close(done)
			}()
			// Wait for the goroutine to finish (yielding)
			sl.WaitForDone()
			Eventually(done).Should(BeClosed())
			// Lock is finished
			Expect(sl.Finished()).To(BeTrue())
		})
	})
})
