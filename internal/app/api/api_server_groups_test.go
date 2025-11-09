package api_test

import (
	"fs-access-api/internal/app/ports"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Groups API (unit)", Ordered, func() {
	var (
		apis  ports.ApiServer
		gname = "team-devs"
	)

	BeforeAll(func() {
		apis = newTestServerFromConfig(TestConfigPath)
	})

	AfterAll(func() {
		// best-effort cleanup (ignore error)
		_ = apis.DeleteGroup(gname)
	})

	It("EnsureGroup: create then idempotent", func() {
		g, created, err := apis.EnsureGroup(ports.GroupInfo{
			Groupname: gname,
			GID:       4001,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(g.Groupname).To(Equal(gname))
		Expect(g.GID).To(Equal(uint32(4001)))
		// created may be true on the first call:
		Expect(created).To(BeTrue())

		g2, created2, err := apis.EnsureGroup(ports.GroupInfo{
			Groupname: gname,
			GID:       4001,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(g2.Groupname).To(Equal(gname))
		Expect(g2.GID).To(Equal(uint32(4001)))
		// should be idempotent on same body:
		Expect(created2).To(BeFalse())
	})

	It("EnsureGroup: same name, different gid must not mutate existing gid", func() {
		// Attempt conflicting ensure:
		_, _, err := apis.EnsureGroup(ports.GroupInfo{
			Groupname: gname,
			GID:       4999,
		})
		// Some implementations return a conflict error; others just keep existing.
		// Accept either, but assert final state is unchanged:
		Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("conflict"))))

		curr, err := apis.GetGroup(gname)
		Expect(err).NotTo(HaveOccurred())
		Expect(curr.GID).To(Equal(uint32(4001))) // gid must remain original
	})

	It("UpdateGroup: mutate description", func() {
		err := apis.UpdateGroup(gname, func(g ports.GroupInfo) (ports.GroupInfo, error) {
			desc := "some-description"
			g.Description = &desc
			return g, nil
		})
		Expect(err).NotTo(HaveOccurred())

		got, err := apis.GetGroup(gname)
		Expect(err).NotTo(HaveOccurred())
		Expect(got.Description).NotTo(BeNil())
		Expect(*got.Description).To(Equal("some-description"))
	})

	It("ListGroups: contains the group", func() {
		list, err := apis.ListGroups()
		Expect(err).NotTo(HaveOccurred())

		var found bool
		for _, g := range list {
			if g.Groupname == gname && g.GID == uint32(4001) {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue())
	})

	It("DeleteGroup: removes the group; GetGroup -> not found", func() {
		err := apis.DeleteGroup(gname)
		Expect(err).NotTo(HaveOccurred())

		_, err = apis.GetGroup(gname)
		// Accept either typed not-found error or message containing "not found"
		Expect(err).To(SatisfyAny(
			MatchError(ContainSubstring("not found")),
			HaveOccurred(),
		))
	})

	It("DeleteGroup: idempotent delete", func() {
		// deleting again should not crash; allow not-found as success semantics as long as no panic
		err := apis.DeleteGroup(gname)
		Expect(err).To(SatisfyAny(BeNil(), MatchError(ContainSubstring("not found"))))
	})
})
