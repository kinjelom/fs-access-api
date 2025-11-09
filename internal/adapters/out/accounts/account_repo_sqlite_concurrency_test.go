package accounts

import (
	"context"
	"errors"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	_ "modernc.org/sqlite"
)

var _ = Describe("SQLiteAccountRepository concurrency (multi-instance)", Ordered, func() {
	var (
		repo1  *SQLiteAccountRepository
		repo2  *SQLiteAccountRepository
		group1 = "test1"
		group2 = "test2"
	)

	BeforeAll(func() {
		tmpDir := GinkgoT().TempDir()
		common := config.AccountRepositoryCommonConfig{MinUID: 2000, MinGID: 2000}
		cfg := config.AccountRepositorySqliteConfig{
			DbFilePath:   filepath.Join(tmpDir, "fs-access.db"),
			WriteTimeout: 100 * time.Millisecond,
			QueryTimeout: 100 * time.Millisecond,
		}
		var err error
		repo1, err = NewSQLiteAccountRepository(cfg, common, true)
		Expect(err).ToNot(HaveOccurred())
		repo2, err = NewSQLiteAccountRepository(cfg, common, false)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterAll(func() {
		if repo1 != nil {
			_ = repo1.db.Close()
		}
		if repo2 != nil {
			_ = repo2.db.Close()
		}
	})

	It("allows write and read on all nodes", func(ctx context.Context) {
		_, err := repo1.AddGroup(ports.GroupInfo{
			Groupname: group1,
			GID:       13001,
			Home:      group1,
		})
		if err != nil && !errors.Is(err, ports.ErrAlreadyExists) {
			Fail("cannot add group: " + err.Error())
		}
		_, err = repo2.AddGroup(ports.GroupInfo{
			Groupname: group2,
			GID:       13002,
			Home:      group2,
		})
		if err != nil && !errors.Is(err, ports.ErrAlreadyExists) {
			Fail("cannot add group: " + err.Error())
		}

		timeout := 1 * time.Second
		// all nodes should eventually see the groups.
		Eventually(func() bool {
			u, err := repo1.GetGroup(group1)
			return err == nil && u.Groupname == group1
		}).WithTimeout(timeout).Should(BeTrue(), "node 1 should see group 1 within: "+timeout.String())
		Eventually(func() bool {
			u, err := repo1.GetGroup(group2)
			return err == nil && u.Groupname == group2
		}).WithTimeout(timeout).Should(BeTrue(), "node 1 should see group 2 within: "+timeout.String())

		Eventually(func() bool {
			u, err := repo2.GetGroup(group1)
			return err == nil && u.Groupname == group1
		}).WithTimeout(timeout).Should(BeTrue(), "node 2 should see group 1 within: "+timeout.String())
		Eventually(func() bool {
			u, err := repo2.GetGroup(group2)
			return err == nil && u.Groupname == group2
		}).WithTimeout(timeout).Should(BeTrue(), "node 2 should see group 2 within: "+timeout.String())

	})

})
