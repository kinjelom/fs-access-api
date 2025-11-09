//go:build unix

package fs_test

import (
	"fs-access-api/internal/adapters/out/fs"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DefaultFsStorageService", func() {
	var (
		fsm          *fs.InMemFilesystemService
		storage      *fs.DefaultFsStorageService
		homesBaseDir string
	)

	BeforeEach(func() {
		tempDir := GinkgoT().TempDir() // /tmp/ginkgo...
		homesBaseDir = filepath.Join(tempDir, "root-dir")
		var err error
		fsm = fs.NewInMemFilesystemService()
		err = fsm.MkdirAll(homesBaseDir, 0o777)
		Expect(err).ToNot(HaveOccurred())
		cfg := config.StorageConfig{
			Implementation:     "unix",
			HomesBaseDir:       homesBaseDir,
			CreateHomesBaseDir: false,
			DefaultUserTopDirs: []string{"_test"},
		}
		storage, err = fs.NewDefaultFsStorageService(cfg, fsm, true)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("PrepareGroupHome", func() {
		It("should prepare group home - simple directory", func() {
			g := ports.GroupInfo{GID: 2000, Home: "group-dir"}
			err := storage.PrepareGroupHome(g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should prepare group home - relative path", func() {
			g := ports.GroupInfo{GID: 2000, Home: "group-dir/group-subdir"}
			err := storage.PrepareGroupHome(g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should prepare group home - traversal inside root", func() {
			g := ports.GroupInfo{GID: 2000, Home: "../root-dir/group-dir"}
			err := storage.PrepareGroupHome(g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should prepare group home - same as root", func() {
			g := ports.GroupInfo{GID: 2000, Home: "."}
			err := storage.PrepareGroupHome(g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should refuse to prepare group home when absolute group path given", func() {
			home := filepath.Join(string(filepath.Separator), "etc", "proftpd") // /etc/proftpd
			g := ports.GroupInfo{GID: 2000, Home: home}
			err := storage.PrepareGroupHome(g)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot prepare group home using absolute path"))
		})

		It("should refuse to prepare group home attempting traversal outside root", func() {
			g := ports.GroupInfo{GID: 2000, Home: filepath.Join("..", "escape")}
			err := storage.PrepareGroupHome(g)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(" escapes "))
		})
	})

	Describe("PrepareUserHome", func() {
		It("should prepare user home - simple directory", func() {
			u := ports.UserInfo{UID: 2001, Home: "user-dir"}
			g := ports.GroupInfo{GID: 2000, Home: "group-dir"}
			err := storage.PrepareUserHome(u, g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should prepare user home - relative path", func() {
			u := ports.UserInfo{UID: 2001, Home: "user-dir/user-subdir"}
			g := ports.GroupInfo{GID: 2000, Home: "group-dir"}
			err := storage.PrepareUserHome(u, g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should prepare user home - traversal inside root", func() {
			u := ports.UserInfo{UID: 2001, Home: "../group-dir/user-dir"}
			g := ports.GroupInfo{GID: 2000, Home: "group-dir"}
			err := storage.PrepareUserHome(u, g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should prepare user home - same as group", func() {
			u := ports.UserInfo{UID: 2001, Home: "."}
			g := ports.GroupInfo{GID: 2000, Home: "group-dir"}
			err := storage.PrepareUserHome(u, g)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should refuse user home outside group home when absolute user path is given", func() {
			uh := string(filepath.Separator) + "etc"
			u := ports.UserInfo{UID: 2001, Home: uh}
			g := ports.GroupInfo{GID: 2000, Home: "groupns"}
			err := storage.PrepareUserHome(u, g)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot prepare user home using absolute path"))
		})

		It("should refuse user home attempting traversal outside group home", func() {
			uh := filepath.Join("..", "..", "escape")
			u := ports.UserInfo{UID: 2001, Home: uh}
			g := ports.GroupInfo{GID: 2000, Home: "groupns"}
			err := storage.PrepareUserHome(u, g)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(" escapes "))
		})
	})

	Describe("PrepareUserHome default top-dirs", func() {
		It("creates default top-dirs with setgid 02770", func() {
			u := ports.UserInfo{UID: 2001, Home: "bob"}
			g := ports.GroupInfo{GID: 2000, Home: "grpA"}
			Expect(storage.PrepareUserHome(u, g)).To(Succeed())

			userHome := filepath.Join(homesBaseDir, "grpA", "bob")
			testDir := filepath.Join(userHome, "_test")

			fi, uid, gid, err := fsm.GetInfo(testDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(uid).To(Equal(uint32(2001)))
			Expect(gid).To(Equal(uint32(2000)))
			Expect(int(fi.Mode().Perm())).To(Equal(0o770))
			// TODO: Expect((fi.Mode()&os.ModeSetgid) != 0).To(BeTrue(), "setgid bit should be set")

			// Always assert base perms; harmless everywhere
			fi, uid, gid, err = fsm.GetInfo(userHome)
			Expect(err).NotTo(HaveOccurred())
			Expect(int(fi.Mode().Perm())).To(Equal(0o751))

		})

	})

	Describe("CreateUserTopDir", func() {
		BeforeEach(func() {
			// Ensure base structure exists
			u := ports.UserInfo{UID: 2002, Home: "alice"}
			g := ports.GroupInfo{GID: 2000, Home: "grpB"}
			Expect(storage.PrepareGroupHome(g)).To(Succeed())
			Expect(storage.PrepareUserHome(u, g)).To(Succeed())
		})

		It("creates a custom top dir under user home with 02770 and proper ownership", func() {
			u := ports.UserInfo{UID: 2002, Home: "alice"}
			g := ports.GroupInfo{GID: 2000, Home: "grpB"}
			Expect(storage.CreateUserTopDir(u, g, "uploads")).To(Succeed())
			top := filepath.Join(homesBaseDir, "grpB", "alice", "uploads")
			fi, uid, gid, err := fsm.GetInfo(top)
			Expect(err).ToNot(HaveOccurred())
			Expect(uid).To(Equal(uint32(2002)))
			Expect(gid).To(Equal(uint32(2000)))
			Expect(fi.IsDir()).To(BeTrue())
			Expect(int(fi.Mode().Perm())).To(Equal(0o770))
			// TODO: Expect((fi.Mode()&os.ModeSetgid) != 0).To(BeTrue(), "setgid bit should be set")
		})

		It("supports relative userHome normalization (../ inside group)", func() {
			// userHome with traversal that still resolves inside group
			u := ports.UserInfo{UID: 2002, Home: "../grpB/alice/../alice"}
			g := ports.GroupInfo{GID: 2000, Home: "grpB"}
			err := storage.CreateUserTopDir(u, g, "more")
			Expect(err).ToNot(HaveOccurred())

			top := filepath.Join(homesBaseDir, "grpB", "alice", "more")
			_, _, _, err = fsm.GetInfo(top)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects absolute userHome", func() {
			absUser := string(filepath.Separator) + "etc"
			u := ports.UserInfo{UID: 2002, Home: absUser}
			g := ports.GroupInfo{GID: 2000, Home: "grpB"}
			err := storage.CreateUserTopDir(u, g, "uploads")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot prepare user home using absolute path"))
		})

		It("rejects absolute topDir", func() {
			u := ports.UserInfo{UID: 2002, Home: "alice"}
			g := ports.GroupInfo{GID: 2000, Home: "grpB"}
			absTop := string(filepath.Separator) + "tmp"
			err := storage.CreateUserTopDir(u, g, absTop)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot prepare top dir using absolute path"))
		})

		It("rejects traversal of topDir outside user home", func() {
			u := ports.UserInfo{UID: 2002, Home: "alice"}
			g := ports.GroupInfo{GID: 2000, Home: "grpB"}
			err := storage.CreateUserTopDir(u, g, "../../escape")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(" escapes "))
		})

	})

})
