package fileflag_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/shakefu/gha-debug/pkg/fileflag"
)

func TestFileFlag(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileFlag Suite")
}

func tmpPath() (path string) {
	var err error
	path, err = os.MkdirTemp(os.TempDir(), "gha-debug-*")
	Expect(err).ToNot(HaveOccurred())
	path = filepath.Join(path, "fileflag")
	return
}

func touch(path string) (err error) {
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

func remove(path string) (err error) {
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
	var flagPath string

	// We have to initalize flagPath within the test context, otherwise there is
	// a possible race condition, which would require a lock, and we don't want
	// to put that everywhere
	AfterEach(func() {
		err := remove(flagPath)
		Expect(err).ToNot(HaveOccurred())
	})

	It("initialize fine", func() {
		path := tmpPath()
		flagPath = path
		ff, err := NewFileFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(ff).ToNot(BeNil())
		defer ff.Close()
	})

	It("should detect file creation", func() {
		done := make(chan interface{})
		path := tmpPath()
		flagPath = path

		ff, err := NewFileFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(ff).ToNot(BeNil())

		// Create our file
		go func() {
			defer GinkgoRecover()
			ff.WaitForWatch()
			err := touch(path)
			Expect(err).ToNot(HaveOccurred())
		}()

		// Wait for the file to be created, then remove it
		go func() {
			defer GinkgoRecover()
			ff.WaitForStart()
			err := remove(path)
			Expect(err).ToNot(HaveOccurred())
		}()

		// Watch for state changes
		go func() {
			defer GinkgoRecover()
			ff.Watch()
		}()

		// Wait for the flag to be closed
		go func() {
			defer GinkgoRecover()
			ff.Wait()
			close(done)
		}()

		Eventually(done, 5).Should(BeClosed())
		ff.Close()
	})

	It("should work if the flag file already exists", func() {
		done := make(chan interface{})
		path := tmpPath()
		flagPath = path

		err := touch(path)
		Expect(err).ToNot(HaveOccurred())

		ff, err := NewFileFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(ff).ToNot(BeNil())

		// Wait for the flag to be started then remove it
		go func() {
			defer GinkgoRecover()
			ff.WaitForStart()
			err := remove(path)
			Expect(err).ToNot(HaveOccurred())
		}()

		// Watch for state changes
		go func() {
			defer GinkgoRecover()
			ff.Watch()
		}()

		// Wait for the flag to be closed
		go func() {
			defer GinkgoRecover()
			ff.Wait()
			close(done)
		}()

		Eventually(done, 5).Should(BeClosed())
		ff.Close()
	})
})
