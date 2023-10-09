package fileflag_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/shakefu/gha-debug/pkg/fileflag"
)

func TestFileFlag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileFlag Suite")
}

func TempPath() (path string, err error) {
	path, err = os.MkdirTemp(os.TempDir(), "gha-debug-*")
	return
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
	var path string

	BeforeEach(func() {
		var err error
		path, err = TempPath()
		path = filepath.Join(path, "fileflag")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := Remove(path)
		Expect(err).ToNot(HaveOccurred())
	})

	It("initialize fine", func() {
		ff, err := NewFileFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(ff).ToNot(BeNil())
		defer ff.Close()
	})

	It("should detect file creation", func() {
		done := make(chan interface{})
		watching := make(chan interface{})
		lock := sync.Mutex{}

		ff, err := NewFileFlag(path, &lock)
		Expect(err).ToNot(HaveOccurred())
		Expect(ff).ToNot(BeNil())

		By("scheduling creation")
		// Create our file
		go func() {
			defer GinkgoRecover()
			By("waiting for watch to start")
			Eventually(watching).Should(BeClosed())
			lock.Lock()
			By(fmt.Sprintf("creating flag=%s", path))
			err := Touch(path)
			By("created")
			lock.Unlock()
			Expect(err).ToNot(HaveOccurred())
		}()

		By("scheduling removal")
		// Wait for the file to be created, then remove it
		go func() {
			defer GinkgoRecover()
			By("waiting for start")
			ff.WaitForStart()
			By("started")
			By("removing flag")
			err := Remove(path)
			By("removed")
			Expect(err).ToNot(HaveOccurred())
		}()

		By("scheduling watch")
		// Watch for state changes
		go func() {
			defer GinkgoRecover()
			By("closing watching channel")
			close(watching)
			By("watching")
			ff.Watch()
			By("closing done")
			close(done)
		}()

		By("waiting for done")
		Eventually(done, 5).Should(BeClosed())
		By("closing FileFlag")
		ff.Close()
	})
})
