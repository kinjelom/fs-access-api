package api_test

import (
	"fs-access-api/internal/app"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func ptr[T any](v T) *T { return &v }

// --- Seedable server ---
func newTestServerFromConfig(configPath string) ports.ApiServer {
	data, err := os.ReadFile(configPath)
	Expect(err).NotTo(HaveOccurred())

	tmpDir := filepath.Join(GinkgoT().TempDir(), "fs-access-api-test")
	err = os.MkdirAll(tmpDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	dataStr := string(data)
	dataStr = strings.ReplaceAll(dataStr, "TEST_TEMP_DIR_PLACEHOLDER", tmpDir)

	cfg, err := config.LoadConfigString(dataStr)
	Expect(err).NotTo(HaveOccurred())

	err = os.MkdirAll(cfg.Storage.HomesBaseDir, 0755)
	Expect(err).NotTo(HaveOccurred())

	rs, err := app.BuildApiServer(cfg, true)
	Expect(err).NotTo(HaveOccurred())

	return rs
}
