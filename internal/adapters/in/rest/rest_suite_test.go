package rest_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const TestConfigPath = "../../../../config.test.yml"

func TestRestServer(t *testing.T) {
	RegisterFailHandler(AbortSuite)
	RunSpecs(t, "REST Suite")
}
