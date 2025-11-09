package security_test

import (
	"bytes"
	"fs-access-api/internal/adapters/out/security"
	"fs-access-api/internal/app/config"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newBearerRequest(method, url string, body []byte, apiKeyID, secretHex string) *http.Request {
	var rdr io.ReadCloser
	if body != nil {
		rdr = io.NopCloser(bytes.NewReader(body))
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("X-Api-Key", apiKeyID)
	req.Header.Set("Authorization", "Bearer "+secretHex)
	return req
}

// --- tests ---

var _ = Describe("BearerAuthenticator.Verify", func() {
	const (
		apiKeyID  = "test-key"
		secretHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	)

	var auth *security.BearerAuthenticator

	BeforeEach(func() {
		sec := config.AuthenticatorConfig{
			WindowSeconds: 300,
			AccessKeys:    map[string]string{apiKeyID: secretHex},
		}
		var err error
		auth, err = security.NewBearerAuthenticator(sec)
		Expect(err).NotTo(HaveOccurred())
	})

	It("accepts a valid signature", func() {
		body := []byte(`{"hello":"world"}`)
		req := newBearerRequest(http.MethodPost, "http://example.test/api/users?x=1", body, apiKeyID, secretHex)

		err := auth.Verify(req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects when required headers are missing", func() {
		req, _ := http.NewRequest(http.MethodGet, "http://example.test/api/users", nil)
		err := auth.Verify(req)
		Expect(err).To(HaveOccurred())
	})

	It("rejects when signature is invalid", func() {
		req, _ := http.NewRequest(http.MethodGet, "http://example.test/api/users", nil)
		req.Header.Set("X-Api-Key", apiKeyID)
		req.Header.Set("Authorization", "Bearer deadbeef")

		err := auth.Verify(req)
		Expect(err).To(HaveOccurred())
	})

})

var _ = Describe("BearerAuthenticator.WithAuthChi middleware", func() {
	const (
		apiKeyID  = "test-key"
		secretHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	)

	var auth *security.BearerAuthenticator
	var router *chi.Mux

	BeforeEach(func() {
		sec := config.AuthenticatorConfig{
			WindowSeconds: 300,
			AccessKeys:    map[string]string{apiKeyID: secretHex},
		}
		var err error
		auth, err = security.NewBearerAuthenticator(sec)
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

	It("allows request and calls next handler when secret is valid", func() {
		req := newBearerRequest(http.MethodGet, "http://example.test/protected", nil, apiKeyID, secretHex)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("X-Next-Called")).To(Equal("1"))
		Expect(rr.Body.String()).To(Equal("ok"))
	})

	It("rejects request with 401 and does not call next handler when secret is invalid", func() {
		req, _ := http.NewRequest(http.MethodGet, "http://example.test/protected", nil)
		req.Header.Set("X-Api-Key", apiKeyID)
		req.Header.Set("Authorization", "Bearer deadbeef")

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
