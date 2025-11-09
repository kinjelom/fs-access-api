package security_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fs-access-api/internal/adapters/out/security"
	"fs-access-api/internal/app/config"
	"io"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// --- helpers ---

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func mustDecodeHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

func signHMAC(method, pathWithQuery string, ts string, body []byte, secretHex string) string {
	// canonical: METHOD \n PATH(+query) \n TIMESTAMP \n BODY_SHA256_HEX
	bodyHash := sha256Hex(body)
	msg := method + "\n" + pathWithQuery + "\n" + ts + "\n" + bodyHash
	key := mustDecodeHex(secretHex)
	m := hmac.New(sha256.New, key)
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

func newHmacSignedRequest(method, url string, body []byte, apiKeyID, secretHex, ts string) *http.Request {
	var rdr io.ReadCloser
	if body != nil {
		rdr = io.NopCloser(bytes.NewReader(body))
	}
	req, _ := http.NewRequest(method, url, rdr)
	if body == nil {
		body = []byte{}
	}
	bodyHash := sha256Hex(body)
	req.Header.Set("X-Api-Key", apiKeyID)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Content-Sha256", bodyHash)

	// Build path+query for signature (same as server uses)
	pathWithQuery := req.URL.EscapedPath()
	if q := req.URL.RawQuery; q != "" {
		pathWithQuery += "?" + q
	}
	sig := signHMAC(method, pathWithQuery, ts, body, secretHex)
	req.Header.Set("Authorization", "HMAC "+sig)
	return req
}

// --- tests ---

var _ = Describe("HMACAuthenticator.Verify", func() {
	const (
		apiKeyID  = "test-key"
		secretHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	)

	var auth *security.HMACAuthenticator

	BeforeEach(func() {
		sec := config.AuthenticatorConfig{
			WindowSeconds: 300,
			AccessKeys:    map[string]string{apiKeyID: secretHex},
		}
		var err error
		auth, err = security.NewHMACAuthenticator(sec)
		Expect(err).NotTo(HaveOccurred())
	})

	It("accepts a valid signature within the time window", func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		body := []byte(`{"hello":"world"}`)
		req := newHmacSignedRequest(http.MethodPost, "http://example.test/api/users?x=1", body, apiKeyID, secretHex, ts)

		err := auth.Verify(req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects when required headers are missing", func() {
		req, _ := http.NewRequest(http.MethodGet, "http://example.test/api/users", nil)
		err := auth.Verify(req)
		Expect(err).To(HaveOccurred())
	})

	It("rejects when signature is invalid", func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		req, _ := http.NewRequest(http.MethodGet, "http://example.test/api/users", nil)
		req.Header.Set("X-Api-Key", apiKeyID)
		req.Header.Set("X-Timestamp", ts)
		req.Header.Set("X-Content-Sha256", sha256Hex([]byte{}))
		req.Header.Set("Authorization", "HMAC deadbeef")

		err := auth.Verify(req)
		Expect(err).To(HaveOccurred())
	})

	It("rejects when timestamp is outside the allowed window", func() {
		old := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
		req := newHmacSignedRequest(http.MethodGet, "http://example.test/api/users", nil, apiKeyID, secretHex, old)

		err := auth.Verify(req)
		Expect(err).To(HaveOccurred())
	})

	It("rejects when X-Content-Sha256 doesn't match the actual body", func() {
		ts := time.Now().UTC().Format(time.RFC3339)

		// Prepare request with mismatched body/hash
		body := []byte(`{"foo":"bar"}`)
		req, _ := http.NewRequest(http.MethodPost, "http://example.test/api/users", io.NopCloser(bytes.NewReader(body)))

		// Intentionally wrong body hash and signature built over the wrong hash
		req.Header.Set("X-Api-Key", apiKeyID)
		req.Header.Set("X-Timestamp", ts)
		req.Header.Set("X-Content-Sha256", sha256Hex([]byte("WRONG")))
		// Build canonical with the WRONG body hash so Authorization is also inconsistent with true body
		path := req.URL.EscapedPath()
		sig := signHMAC(http.MethodPost, path, ts, []byte("WRONG"), secretHex)
		req.Header.Set("Authorization", "HMAC "+sig)

		err := auth.Verify(req)
		Expect(err).To(HaveOccurred())
	})

	It("accepts empty body when its SHA-256 is the empty-body digest", func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		req := newHmacSignedRequest(http.MethodGet, "http://example.test/api/users", nil, apiKeyID, secretHex, ts)

		err := auth.Verify(req)
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("HMACAuthenticator.WithAuthChi middleware", func() {
	const (
		apiKeyID  = "test-key"
		secretHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	)

	var auth *security.HMACAuthenticator
	var router *chi.Mux

	BeforeEach(func() {
		sec := config.AuthenticatorConfig{
			WindowSeconds: 300,
			AccessKeys:    map[string]string{apiKeyID: secretHex},
		}
		var err error
		auth, err = security.NewHMACAuthenticator(sec)
		Expect(err).NotTo(HaveOccurred())

		router = chi.NewRouter()
		// Protect only this endpoint with WithAuthChi
		protected := chi.NewRouter()
		protected.Use(auth.WithAuthChi)
		protected.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Next-Called", "1")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		router.Mount("/", protected)

		// Public endpoint to sanity-check router
		router.Get("/public", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("public"))
		})
	})

	It("allows request and calls next handler when signature is valid", func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		req := newHmacSignedRequest(http.MethodGet, "http://example.test/protected", nil, apiKeyID, secretHex, ts)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("X-Next-Called")).To(Equal("1"))
		Expect(rr.Body.String()).To(Equal("ok"))
	})

	It("rejects request with 401 and does not call next handler when signature is invalid", func() {
		req, _ := http.NewRequest(http.MethodGet, "http://example.test/protected", nil)
		req.Header.Set("X-Api-Key", apiKeyID)
		req.Header.Set("X-Timestamp", time.Now().UTC().Format(time.RFC3339))
		req.Header.Set("X-Content-Sha256", sha256Hex([]byte{}))
		req.Header.Set("Authorization", "HMAC deadbeef")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
		Expect(rr.Header().Get("X-Next-Called")).To(BeEmpty(), "next handler must not be invoked")
	})

	It("does not protect public routes if not wrapped", func() {
		req, _ := http.NewRequest(http.MethodGet, "http://example.test/public", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("public"))
	})
})
