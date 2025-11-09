package fs_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFS(t *testing.T) {
	RegisterFailHandler(AbortSuite)
	RunSpecs(t, "FS Suite")
}
