package rest_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fs-access-api/internal/adapters/in/rest/openapi"
)

var _ = Describe("Groups REST E2E (smoke)", func() {
	var (
		ctx   = context.Background()
		base  string
		cli   *openapi.ClientWithResponses
		group = "team-devs"
	)

	BeforeEach(func() {
		s := newTestServerFromConfig(TestConfigPath)
		base = s.URL
		cli = newHmacClient(base, apiKeyID, secretHex)
		DeferCleanup(s.Close)
	})

	It("ensure(idempotent) -> get -> delete -> get404", func() {
		// ensure (create or ok)
		ens1, err := cli.EnsureGroupWithResponse(ctx, group, openapi.EnsureGroupRequestBody{Gid: 4001})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ens1.StatusCode(), ens1.Body, http.StatusCreated, http.StatusOK)

		// ensure again (idempotent)
		ens2, err := cli.EnsureGroupWithResponse(ctx, group, openapi.EnsureGroupRequestBody{Gid: 4001})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ens2.StatusCode(), ens2.Body, http.StatusOK)

		// get
		get, err := cli.GetGroupWithResponse(ctx, group)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(get.StatusCode(), get.Body, http.StatusOK)
		Expect(get.JSON200.Groupname).To(Equal(group))
		Expect(get.JSON200.Gid).To(Equal(uint32(4001)))

		// delete
		del, err := cli.DeleteGroupWithResponse(ctx, group)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(del.StatusCode(), del.Body, http.StatusNoContent, http.StatusOK)

		// get -> 404
		get2, err := cli.GetGroupWithResponse(ctx, group)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(get2.StatusCode(), get2.Body, http.StatusNotFound)
	})
})
