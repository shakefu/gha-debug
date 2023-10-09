package fileflag_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/shakefu/gha-debug/pkg/fileflag"
)

func TestFileFlag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileFlag Suite")
}

func Touch(path string) (err error) {
	// Ensure the directory exists
	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return
	}
	// Create the file
	_, err = os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		_, err = os.Create(path)
	}
	return
}

func Remove(path string) (err error) {
	_, err = os.Stat(path)
	if err == nil {
		err = os.Remove(path)
	} else if !os.IsNotExist(err) {
		// Return the error if it's something other than not existing
		return
	} else if os.IsNotExist(err) {
		// Not an error if it doesn't exist
		err = nil
	}
	return
}

var _ = Describe("FileFlag", func() {
	// TODO: Use unique name
	path := "/tmp/gha-debug/test"

	AfterEach(func() {
		err := Remove(path)
		Expect(err).ToNot(HaveOccurred())
	})

	It("initialize fine", func() {
		ff, err := NewFileFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(ff).ToNot(BeNil())
	})

	It("should detect file creation", func() {
		done := make(chan interface{})

		By("new instance")
		ff, err := NewFileFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(ff).ToNot(BeNil())

		go func() {
			defer GinkgoRecover()
			By("creating flag file")
			err := Touch(path)
			Expect(err).ToNot(HaveOccurred())
		}()

		go func() {
			defer GinkgoRecover()
			By("waiting for flag to start")
			ff.WaitForStart()
			By("removing flag file")
			err := Remove(path)
			Expect(err).ToNot(HaveOccurred())
			By("flag file removed")
		}()

		go func() {
			defer GinkgoRecover()
			By("watching for flag file changes")
			ff.Watch()
			By("closing done channel")
			close(done)
		}()

		By("deferring execution")
		runtime.Gosched()

		Eventually(done).Should(BeClosed())
	})

	// It("shouldn't error when watching", func() {
	// 	ff, err := NewFileFlag(path)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	Expect(ff).ToNot(BeNil())
	// 	By("scheduling file creation")
	// 	go func() {
	// 		defer GinkgoRecover()
	// 		os.Create(path)
	// 	}()
	// 	By("watching for file creation")
	// 	ff.Watch()
	// 	By("waiting for lock to be released")
	// 	Expect(false).To(BeTrue())
	// })
})
