package rest_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fs-access-api/internal/adapters/out/metrics"
	"fs-access-api/internal/app"
	"fs-access-api/internal/app/config"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fs-access-api/internal/adapters/in/rest/openapi"
)

func ptr[T any](v T) *T { return &v }

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

const secretHex = "77f280ba374a80132dfe7ddaba5af72476be5ba34477448fff901ebc804e4b1e"
const apiKeyID = "key1"
const securityWindowSeconds = 100

func mustStatus(code int, body []byte, allowed ...int) {
	for _, a := range allowed {
		if code == a {
			return
		}
	}
	Expect(code).To(BeElementOf(allowed), "status=%d body=%s", code, string(body))
}

// --- Seedable server ---

func newTestServerFromConfig(configPath string) *httptest.Server {
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

	rs, err := app.BuildRestServer(cfg, true, &metrics.FakeActionMetrics{})
	Expect(err).NotTo(HaveOccurred())

	r := chi.NewRouter()
	_ = openapi.HandlerFromMux(rs, r)
	return httptest.NewServer(r)
}

// Bearer client
func newBearerClient(baseURL, apiKeyID, secretHex string) *openapi.ClientWithResponses {
	editor := func(ctx context.Context, req *http.Request) error {
		path := req.URL.EscapedPath()
		if q := req.URL.RawQuery; q != "" {
			path += "?" + q
		}
		req.Header.Set("X-Api-Key", apiKeyID)
		req.Header.Set("Authorization", "Bearer "+secretHex)
		return nil
	}
	cli, err := openapi.NewClientWithResponses(baseURL, openapi.WithRequestEditorFn(editor))
	Expect(err).NotTo(HaveOccurred())
	return cli
}

// Signed client
func newHmacClient(baseURL, apiKeyID, secretHex string) *openapi.ClientWithResponses {
	editor := func(ctx context.Context, req *http.Request) error {
		var body []byte
		if req.Body != nil {
			b, _ := io.ReadAll(req.Body)
			body = b
			_ = req.Body.Close()
			req.Body = io.NopCloser(bytes.NewReader(b))
		}
		bodyHash := sha256Hex(body)
		ts := time.Now().UTC().Format(time.RFC3339)

		path := req.URL.EscapedPath()
		if q := req.URL.RawQuery; q != "" {
			path += "?" + q
		}
		msg := req.Method + "\n" + path + "\n" + ts + "\n" + bodyHash

		key, _ := hex.DecodeString(secretHex)
		m := hmac.New(sha256.New, key)
		_, _ = m.Write([]byte(msg))
		sig := hex.EncodeToString(m.Sum(nil))

		req.Header.Set("X-Api-Key", apiKeyID)
		req.Header.Set("X-Timestamp", ts)
		req.Header.Set("X-Content-Sha256", bodyHash)
		req.Header.Set("Authorization", "HMAC "+sig)
		return nil
	}
	cli, err := openapi.NewClientWithResponses(baseURL, openapi.WithRequestEditorFn(editor))
	Expect(err).NotTo(HaveOccurred())
	return cli
}
