package rest_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fs-access-api/internal/adapters/in/rest/openapi"
)

var _ = Describe("Crypto REST E2E (smoke)", func() {
	var (
		ctx    = context.Background()
		srvURL string
		pub    *openapi.ClientWithResponses
	)

	BeforeEach(func() {
		s := newTestServerFromConfig(TestConfigPath)
		srvURL = s.URL
		var err error
		pub, err = openapi.NewClientWithResponses(srvURL)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(s.Close)
	})

	It("POST /api/hash: sha256 with rounds -> $5$ prefix", func() {
		body := openapi.ComputeHashRequestBody{
			Algorithm: openapi.CryptSha256,
			Rounds:    ptr(5000),
			SaltLen:   ptr(8),
			Plaintext: ptr("secret"),
		}
		res, err := pub.ComputeHashWithResponse(ctx, body)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(res.StatusCode(), res.Body, http.StatusOK)
		Expect(res.JSON200.Hash).To(HavePrefix("$5$rounds=5000$"))
	})

	It("POST /api/hash: invalid rounds (<1000) -> 400", func() {
		body := openapi.ComputeHashRequestBody{
			Algorithm: openapi.CryptSha512,
			Rounds:    ptr(999),
			SaltLen:   ptr(8),
			Plaintext: ptr("x"),
		}
		res, err := pub.ComputeHashWithResponse(ctx, body)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(res.StatusCode(), res.Body, http.StatusBadRequest)
	})

	It("POST /api/verify: good and bad password", func() {
		h, err := pub.ComputeHashWithResponse(ctx, openapi.ComputeHashRequestBody{
			Algorithm: openapi.CryptSha256, Rounds: ptr(5000), SaltLen: ptr(16), Plaintext: ptr("p@ss"),
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(h.StatusCode(), h.Body, http.StatusOK)

		ok, err := pub.VerifyHashWithResponse(ctx, openapi.VerifyHashRequestBody{
			Hash: h.JSON200.Hash, Plaintext: ptr("p@ss"),
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ok.StatusCode(), ok.Body, http.StatusOK)
		Expect(ok.JSON200.Verified).To(BeTrue())

		bad, err := pub.VerifyHashWithResponse(ctx, openapi.VerifyHashRequestBody{
			Hash: h.JSON200.Hash, Plaintext: ptr("nope"),
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(bad.StatusCode(), bad.Body, http.StatusOK)
		Expect(bad.JSON200.Verified).To(BeFalse())
	})

	It("GET /api/secret: explicit size and default=32", func() {
		r16, _ := pub.GenerateSecretWithResponse(ctx, &openapi.GenerateSecretParams{Size: ptr(16)})
		mustStatus(r16.StatusCode(), r16.Body, http.StatusOK)
		Expect(r16.JSON200.SizeBytes).To(Equal(16))

		rDef, _ := pub.GenerateSecretWithResponse(ctx, nil)
		mustStatus(rDef.StatusCode(), rDef.Body, http.StatusOK)
		Expect(rDef.JSON200.SizeBytes).To(Equal(32))
	})
})
