package rest_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"fs-access-api/internal/adapters/in/rest/openapi"
)

var _ = Describe("Users REST E2E (smoke)", Ordered, func() {
	var (
		ctx        = context.Background()
		baseURL    string
		cli        *openapi.ClientWithResponses
		badAuthCli *openapi.ClientWithResponses
	)

	const user = "bob"
	const passwd = "Secr3t!"

	BeforeAll(func() {
		s := newTestServerFromConfig(TestConfigPath)
		baseURL = s.URL
		cli = newHmacClient(baseURL, apiKeyID, secretHex)
		badAuthCli = newHmacClient(baseURL, apiKeyID, secretHex+"deadbeef")
		DeferCleanup(s.Close)
	})

	It("1) ensure(create/idempotent) and basic auth works", func() {
		req := openapi.EnsureUserRequestBody{
			Groupname: "default",
			Home:      ptr("bob-home"),
			Password:  ptr(passwd),
			// plaintext provided:
			PasswordIsHash: ptr(false),
			Description:    ptr("Bob"),
		}
		ens1, _ := cli.EnsureUserWithResponse(ctx, user, req)
		mustStatus(ens1.StatusCode(), ens1.Body, http.StatusCreated, http.StatusOK)

		ens2, _ := cli.EnsureUserWithResponse(ctx, user, req) // idempotent
		mustStatus(ens2.StatusCode(), ens2.Body, http.StatusOK)

		ver, err := cli.AuthzAuthUserWithFormdataBodyWithResponse(ctx, user, openapi.AuthzAuthUserFormdataRequestBody{
			Password: passwd,
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ver.StatusCode(), ver.Body, http.StatusNoContent)
	})

	It("2) unauthorized API client -> 401", func() {
		ver, err := badAuthCli.AuthzAuthUserWithFormdataBodyWithResponse(ctx, user, openapi.AuthzAuthUserFormdataRequestBody{
			Password: passwd,
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ver.StatusCode(), ver.Body, http.StatusUnauthorized)
	})

	It("3) disable -> locked; enable -> ok", func() {
		d1, err := cli.SetUserDisabledWithResponse(ctx, user, openapi.SetUserDisabledRequestBody{Disabled: true})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(d1.StatusCode(), d1.Body, http.StatusNoContent)

		locked, err := cli.AuthzAuthUserWithFormdataBodyWithResponse(ctx, user, openapi.AuthzAuthUserFormdataRequestBody{
			Password: passwd,
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(locked.StatusCode(), locked.Body, http.StatusLocked)

		d2, err := cli.SetUserDisabledWithResponse(ctx, user, openapi.SetUserDisabledRequestBody{Disabled: false})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(d2.StatusCode(), d2.Body, http.StatusNoContent)

		ok, err := cli.AuthzAuthUserWithFormdataBodyWithResponse(ctx, user, openapi.AuthzAuthUserFormdataRequestBody{
			Password: passwd,
		})
		Expect(err).NotTo(HaveOccurred())
		mustStatus(ok.StatusCode(), ok.Body, http.StatusNoContent)
	})

	It("4) delete -> get 404", func() {
		del, err := cli.DeleteUserWithResponse(ctx, user)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(del.StatusCode(), del.Body, http.StatusNoContent, http.StatusOK)

		get, err := cli.GetUserWithResponse(ctx, user)
		Expect(err).NotTo(HaveOccurred())
		mustStatus(get.StatusCode(), get.Body, http.StatusNotFound)
	})
})
