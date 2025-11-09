package security_test

import (
	"fs-access-api/internal/adapters/out/security"
	"fs-access-api/internal/app/config"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MultiAuthenticator.Verify", func() {
	const (
		apiKeyID  = "test-key"
		secretHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	)

	var auth *security.MultiAuthenticator

	BeforeEach(func() {
		sec := config.AuthenticatorConfig{
			EnabledAuthenticators: []string{"bearer", "hmac"},
			WindowSeconds:         300,
			AccessKeys:            map[string]string{apiKeyID: secretHex},
		}
		var err error
		auth, err = security.NewMultiAuthenticator(sec)
		Expect(err).NotTo(HaveOccurred())
	})

	It("accepts a valid signature", func() {
		body := []byte(`{"hello":"world"}`)
		req := newBearerRequest(http.MethodPost, "http://example.test/api/users?x=1", body, apiKeyID, secretHex)

		err := auth.Verify(req)
		Expect(err).NotTo(HaveOccurred())
	})

	It("accepts a valid signature within the time window", func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		body := []byte(`{"hello":"world"}`)
		req := newHmacSignedRequest(http.MethodPost, "http://example.test/api/users?x=1", body, apiKeyID, secretHex, ts)

		err := auth.Verify(req)
		Expect(err).NotTo(HaveOccurred())
	})

})

var _ = Describe("BearerAuthenticator.WithAuthChi middleware", func() {
	const (
		apiKeyID  = "test-key"
		secretHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	)

	var auth *security.MultiAuthenticator
	var router *chi.Mux

	BeforeEach(func() {
		sec := config.AuthenticatorConfig{
			EnabledAuthenticators: []string{"bearer", "hmac"},
			WindowSeconds:         300,
			AccessKeys:            map[string]string{apiKeyID: secretHex},
		}
		var err error
		auth, err = security.NewMultiAuthenticator(sec)
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

	It("allows request and calls next handler when signature is valid", func() {
		ts := time.Now().UTC().Format(time.RFC3339)
		req := newHmacSignedRequest(http.MethodGet, "http://example.test/protected", nil, apiKeyID, secretHex, ts)

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("X-Next-Called")).To(Equal("1"))
		Expect(rr.Body.String()).To(Equal("ok"))
	})

})
