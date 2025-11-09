package api_test

import (
	"errors"
	"fs-access-api/internal/app/ports"
	"time"

	uuid2 "github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Users API (unit)", Ordered, func() {
	var apis ports.ApiServer
	const user = "bob"
	const passwd = "Secr3t!"

	BeforeAll(func() {
		apis = newTestServerFromConfig(TestConfigPath)
	})

	AfterAll(func() {
		_ = apis.DeleteUser(user) // best-effort cleanup
	})

	It("EnsureUser: create then idempotent", func() {
		u, created, err := apis.EnsureUser(ports.UserInfo{
			Username:       user,
			Groupname:      "default",
			Home:           "bob-home",
			Description:    ptr("Bob"),
			Password:       passwd,
			PasswordIsHash: false,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(u.Username).To(Equal(user))
		Expect(created).To(BeTrue())

		u2, created2, err := apis.EnsureUser(ports.UserInfo{
			Username:       user,
			Groupname:      "default",
			Home:           "bob-home",
			Description:    ptr("Bob"),
			Password:       passwd,
			PasswordIsHash: false,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(u2.Username).To(Equal(user))
		Expect(created2).To(BeFalse())
	})

	It("EnsureUser: conflicting properties (e.g., different home/gid) -> conflict or preserved state", func() {
		_, _, err := apis.EnsureUser(ports.UserInfo{
			Username:  user,
			Groupname: "default",
			Home:      "/other/home", // conflicts with original
		})
		// Accept either a typed conflict error or preserved state without mutation:
		Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("conflict"))))

		got, err := apis.GetUser(user)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Home).To(Equal("bob-home")) // must remain original
	})

	It("Set password using precomputed hash (PasswordIsHash=true)", func() {
		// For unit level: compute via server’s hash API, then set.
		hash, err := apis.ComputeHash(passwd, ports.AlgoRawSHA256, nil, nil)
		Expect(err).NotTo(HaveOccurred())

		err = apis.UpdateUser(user, func(u ports.UserInfo) (ports.UserInfo, error) {
			u.Password = hash
			u.PasswordIsHash = true
			return u, nil
		})
		Expect(err).NotTo(HaveOccurred())

		// Auth should still pass (server must interpret raw hash correctly per implementation contract)
		err = apis.AuthzAuthUser(user, passwd)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Set and clear expiration", func() {
		// Expire in the past
		past := time.Now().UTC().Add(-time.Hour)
		err := apis.UpdateUser(user, func(u ports.UserInfo) (ports.UserInfo, error) {
			u.Expiration = &past
			return u, nil
		})
		Expect(err).NotTo(HaveOccurred())

		err = apis.AuthzAuthUser(user, passwd)
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ports.ErrLockedUser)).To(BeTrue())

		// Clear expiration
		err = apis.UpdateUser(user, func(u ports.UserInfo) (ports.UserInfo, error) {
			u.Expiration = nil
			return u, nil
		})
		Expect(err).NotTo(HaveOccurred())

		err = apis.AuthzAuthUser(user, passwd)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Disable and enable user", func() {
		err := apis.UpdateUser(user, func(u ports.UserInfo) (ports.UserInfo, error) {
			u.Disabled = true
			return u, nil
		})
		Expect(err).NotTo(HaveOccurred())

		err = apis.AuthzAuthUser(user, passwd)
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ports.ErrLockedUser)).To(BeTrue())

		err = apis.UpdateUser(user, func(u ports.UserInfo) (ports.UserInfo, error) {
			u.Disabled = false
			return u, nil
		})
		Expect(err).NotTo(HaveOccurred())

		err = apis.AuthzAuthUser(user, passwd)
		Expect(err).NotTo(HaveOccurred())
	})

	It("ListUsers contains the user", func() {
		list, err := apis.ListUsers()
		Expect(err).NotTo(HaveOccurred())

		var found bool
		for _, u := range list {
			if u.Username == user {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue())
	})

	It("UpdateUser mutate description/home", func() {
		err := apis.UpdateUser(user, func(u ports.UserInfo) (ports.UserInfo, error) {
			u.Home = "bob-home-2"
			desc := "Bob Updated"
			u.Description = &desc
			return u, nil
		})
		Expect(err).NotTo(HaveOccurred())

		got, err := apis.GetUser(user)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Home).To(Equal("bob-home-2"))
		Expect(got.Description).NotTo(BeNil())
		Expect(*got.Description).To(Equal("Bob Updated"))
	})

	It("DeleteUser then GetUser -> not found; idempotent delete", func() {
		err := apis.DeleteUser(user)
		Expect(err).NotTo(HaveOccurred())

		_, err = apis.GetUser(user)
		Expect(err).To(SatisfyAny(
			MatchError(ContainSubstring("not found")),
			HaveOccurred(),
		))

		// idempotent
		err = apis.DeleteUser(user)
		Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("not found"))))
	})

	It("ListUserDirs -> not found", func() {
		_, err := apis.ListUserDirs(user)
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, ports.ErrNotFound)).To(BeTrue())
	})

	It("ListUserDirs -> found _test", func() {
		u, created, err := apis.EnsureUser(ports.UserInfo{
			Username:       user,
			Groupname:      "default",
			Home:           "bob-home",
			Description:    ptr("Bob"),
			Password:       passwd,
			PasswordIsHash: false,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(u.Username).To(Equal(user))
		Expect(created).To(BeTrue())

		dirs, err := apis.ListUserDirs(user)
		Expect(err).NotTo(HaveOccurred())
		Expect(dirs).To(ConsistOf("_test"))
	})

	It("EnsureUserDir -> found _test; idempotent delete", func() {
		dirName := "test-temp" + uuid2.New().String()
		created, err := apis.EnsureUserDir(user, dirName)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())

		dirs, err := apis.ListUserDirs(user)
		Expect(err).NotTo(HaveOccurred())
		Expect(dirs).To(ConsistOf("_test", dirName))

		err = apis.DeleteUserDir(user, dirName)
		Expect(err).NotTo(HaveOccurred())
		dirs, err = apis.ListUserDirs(user)
		Expect(err).NotTo(HaveOccurred())
		Expect(dirs).To(ConsistOf("_test"))

	})
})
