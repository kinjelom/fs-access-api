package api_test

import (
	"errors"
	"fs-access-api/internal/app/ports"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Authz API (unit)", Ordered, func() {
	var apis ports.ApiServer

	BeforeAll(func() {
		apis = newTestServerFromConfig(TestConfigPath)
	})

	var _ = Describe("AuthzAuthUser", func() {
		It("authorizes an active user", func() {
			err := apis.AuthzAuthUser("operator-a", "test")
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects bad password as invalid credentials", func() {
			err := apis.AuthzAuthUser("operator-a", "test-wrong")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ports.ErrInvalidCredentials)).To(BeTrue())
		})

		It("rejects expired user as locked", func() {
			err := apis.AuthzAuthUser("user-a1", "test")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ports.ErrLockedUser)).To(BeTrue())
		})

		It("rejects disabled user as locked", func() {
			err := apis.AuthzAuthUser("user-a2", "test")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ports.ErrLockedUser)).To(BeTrue())
		})

		It("rejects empty password as invalid input", func() {
			err := apis.AuthzAuthUser("operator-a", "")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ports.ErrInvalidInput)).To(BeTrue())
		})

		It("treats unknown user as invalid credentials (no user enumeration)", func() {
			err := apis.AuthzAuthUser("unknown-user", "whatever")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, ports.ErrInvalidCredentials)).To(BeTrue())
		})
	})
	Describe("AuthzLookupUser", func() {
		It("existing user -> returns UID/GID/Home via UserAuthzInfo", func() {
			uai, rootPath, err := apis.AuthzLookupUser("operator-a")
			Expect(err).NotTo(HaveOccurred())
			Expect(uai.UID).To(Equal(uint32(2001)))
			Expect(uai.GID).To(Equal(uint32(4001)))
			Expect(uai.UserHome).To(HaveSuffix("."))
			Expect(uai.GroupHome).To(HaveSuffix("a"))
			Expect(rootPath).To(HaveSuffix("/fs-access-api-test/homes"))
		})

		It("non-existing user -> uai==nil and no crash", func() {
			uai, rootPath, err := apis.AuthzLookupUser("operator-x")
			Expect(err).To(HaveOccurred())
			Expect(uai).To(BeNil())
			Expect(rootPath).To(HaveSuffix(""))
		})
	})
})
