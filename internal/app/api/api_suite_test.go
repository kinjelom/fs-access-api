package api_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const TestConfigPath = "../../../config.test.yml"

func TestApiServer(t *testing.T) {
	RegisterFailHandler(AbortSuite)
	RunSpecs(t, "API Suite")
}
