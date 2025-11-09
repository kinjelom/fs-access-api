package rest_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fs-access-api/internal/adapters/in/rest/openapi"
)

var _ = Describe("Authz REST E2E (smoke)", Ordered, func() {
	var (
		ctx        = context.Background()
		baseURL    string
		authCli    *openapi.ClientWithResponses
		badAuthCli *openapi.ClientWithResponses
	)

	BeforeAll(func() {
		s := newTestServerFromConfig(TestConfigPath)
		baseURL = s.URL
		authCli = newHmacClient(baseURL, apiKeyID, secretHex)
		badAuthCli = newHmacClient(baseURL, apiKeyID, secretHex+"0123")
		DeferCleanup(s.Close)
	})

	It("Auth: happy-path (authorized user/password) -> 204", func() {
		ver, err := authCli.AuthzAuthUserWithFormdataBodyWithResponse(ctx, "operator-a", openapi.AuthzAuthUserFormdataRequestBody{
			Password: "test",
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ver.StatusCode(), ver.Body, http.StatusNoContent)
	})

	It("Auth: API client not authenticated (bad HMAC) -> 401", func() {
		ver, err := badAuthCli.AuthzAuthUserWithFormdataBodyWithResponse(ctx, "operator-a", openapi.AuthzAuthUserFormdataRequestBody{
			Password: "test",
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ver.StatusCode(), ver.Body, http.StatusUnauthorized)
	})

	It("Lookup: happy-path -> 204 + headers", func() {
		resp, err := authCli.AuthzLookupUserWithResponse(ctx, "operator-a")
		Expect(err).NotTo(HaveOccurred())
		mustStatus(resp.StatusCode(), resp.Body, http.StatusNoContent)
		Expect(resp.HTTPResponse.Header.Get("X-FS-UID")).To(Equal("2001"))
		Expect(resp.HTTPResponse.Header.Get("X-FS-GID")).To(Equal("4001"))
		Expect(resp.HTTPResponse.Header.Get("X-FS-Dir")).To(HaveSuffix("/a"))
	})
})
