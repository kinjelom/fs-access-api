package rest_test

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fs-access-api/internal/adapters/in/rest/openapi"
)

var _ = Describe("Authentication variants", func() {
	var sBase string
	var ctx = context.Background()

	BeforeEach(func() {
		s := newTestServerFromConfig(TestConfigPath)
		sBase = s.URL
		DeferCleanup(s.Close)
	})

	It("missing one header -> 401", func() {
		// Use editor which sets only some headers.
		cli, err := openapi.NewClientWithResponses(sBase, openapi.WithRequestEditorFn(
			func(_ context.Context, r *http.Request) error {
				r.Header.Set("X-Api-Key", apiKeyID)
				// Missing others
				return nil
			},
		))
		Expect(err).NotTo(HaveOccurred())

		res, err := cli.ListUsersWithResponse(ctx)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(res.StatusCode(), res.Body, http.StatusUnauthorized)
	})

	It("timestamp skew outside window -> 401", func() {
		cli, err := openapi.NewClientWithResponses(sBase, openapi.WithRequestEditorFn(
			func(_ context.Context, r *http.Request) error {
				r.Header.Set("X-Api-Key", apiKeyID)
				r.Header.Set("X-Timestamp", time.Now().UTC().Add(-time.Duration(securityWindowSeconds+10)*time.Second).Format(time.RFC3339))
				r.Header.Set("X-Content-Sha256", sha256Hex(nil))
				r.Header.Set("Authorization", "HMAC deadbeef")
				return nil
			},
		))
		Expect(err).NotTo(HaveOccurred())

		res, err := cli.ListUsersWithResponse(ctx)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(res.StatusCode(), res.Body, http.StatusUnauthorized)
	})

	It("body hash mismatch -> 401", func() {
		cli, err := openapi.NewClientWithResponses(sBase, openapi.WithRequestEditorFn(
			func(_ context.Context, r *http.Request) error {
				r.Header.Set("X-Api-Key", apiKeyID)
				r.Header.Set("X-Timestamp", time.Now().UTC().Format(time.RFC3339))
				r.Header.Set("X-Content-Sha256", "00") // wrong length/val
				r.Header.Set("Authorization", "HMAC deadbeef")
				return nil
			},
		))
		Expect(err).NotTo(HaveOccurred())
		res, err := cli.ListUsersWithResponse(ctx)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(res.StatusCode(), res.Body, http.StatusUnauthorized)
	})
})
