package main_test

import (
	"testing"

	. "github.com/shakefu/gha-debug"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Main Suite")
}

var _ = Describe("Cli", func() {
	It("should pass", func() {
		cli := Cli{}
		Expect(cli).ToNot(BeNil())
	})
})
