package config_test

import (
	"os"
	"time"

	"fs-access-api/internal/app/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const TestConfigPath = "../../../config.test.yml"

var _ = Describe("LoadConfig", func() {

	When("the file does not exist", func() {
		It("returns an error and nil config", func() {
			cfg, err := config.LoadConfig("this-file-does-not-exist.yaml")
			Expect(err).To(HaveOccurred())
			Expect(cfg).To(BeNil())
		})
	})

	When("the file contains invalid YAML", func() {
		It("returns a parse error and nil config", func() {
			f, err := os.CreateTemp("", "invalid-*.yaml")
			Expect(err).ToNot(HaveOccurred())
			defer func(name string) {
				_ = os.Remove(name)
			}(f.Name())

			_, err = f.WriteString("not valid yaml: : :")
			Expect(err).ToNot(HaveOccurred())
			_ = f.Close()

			cfg, loadErr := config.LoadConfig(f.Name())
			Expect(loadErr).To(HaveOccurred())
			Expect(cfg).To(BeNil())
		})
	})

	When("loading from an in-memory YAML string", func() {
		It("parses, expands env vars and applies defaults", func() {
			// set env for expansion
			Expect(os.Setenv("HOMES_DIR", "/var/lib/fsaa-homes")).To(Succeed())
			defer func() {
				_ = os.Unsetenv("HOMES_DIR")
			}()

			yamlStr := `
storage:
  implementation: unix
  homes_base_dir: ${HOMES_DIR:-/default/homes}
http_server: {}
security:
  authenticator:
    access_keys:
      api1: secret-1
account_repository:
  type: inmem
  common: {}
  inmem: {}
metrics: {}
`
			cfg, err := config.LoadConfigString(yamlStr)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())

			// env expanded
			Expect(cfg.Storage.HomesBaseDir).To(Equal("/var/lib/fsaa-homes"))

			// defaults from struct tags (go-defaults)
			Expect(cfg.Metrics.Namespace).To(Equal("fsaa"))
			Expect(cfg.HttpServer.ListenAddress).To(Equal(":8080"))
			Expect(cfg.HttpServer.TelemetryPath).To(Equal("/metrics"))
			Expect(cfg.Storage.Implementation).To(Equal("unix"))
			// this one had default:"[_test]"
			Expect(cfg.Storage.DefaultUserTopDirs).To(ConsistOf("_test"))

			// authenticator defaults
			Expect(cfg.Security.Authenticator.WindowSeconds).To(Equal(60))
			Expect(cfg.Security.Authenticator.EnabledAuthenticators).
				To(ConsistOf("hmac", "bearer"))

			// repository defaults
			Expect(cfg.AccountRepository.Common.MinUID).To(Equal(uint32(2000)))
			Expect(cfg.AccountRepository.Common.MinGID).To(Equal(uint32(2000)))
			Expect(cfg.AccountRepository.InMem.EntitiesLimit).To(Equal(1000))
		})
	})

	When("YAML uses default part in ${VAR:-default}", func() {
		It("uses the default when env is missing", func() {
			// make sure var is not set
			_ = os.Unsetenv("MISSING_ENV")

			yamlStr := `
storage:
  implementation: unix
  homes_base_dir: ${MISSING_ENV:-/fallback/dir}
account_repository:
  type: inmem
  common: {}
  inmem: {}
http_server: {}
security:
  authenticator: {}
metrics: {}
`
			cfg, err := config.LoadConfigString(yamlStr)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Storage.HomesBaseDir).To(Equal("/fallback/dir"))
		})
	})

	When("real test file exists", func() {
		It("loads it successfully (smoke test)", func() {
			// only run if file is present on developer machine/CI
			if _, err := os.Stat(TestConfigPath); err != nil {
				Skip("test config file not found: " + TestConfigPath)
			}
			cfg, err := config.LoadConfig(TestConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
		})
	})
})

var _ = Describe("ProgramConfig utility methods", func() {
	It("returns secret by key", func() {
		yamlStr := `
storage: { implementation: unix }
http_server: {}
metrics: {}
security:
  authenticator:
    access_keys:
      keyA: valA
account_repository:
  type: inmem
  common: {}
  inmem: {}
`
		cfg, err := config.LoadConfigString(yamlStr)
		Expect(err).ToNot(HaveOccurred())

		secret, err := cfg.GetSecretKey("keyA")
		Expect(err).ToNot(HaveOccurred())
		Expect(secret).To(Equal("valA"))

		_, err = cfg.GetSecretKey("nope")
		Expect(err).To(HaveOccurred())
	})

	It("maps initial users from YAML keys into Username field", func() {
		yamlStr := `
storage: { implementation: unix }
http_server: {}
metrics: {}
security: { authenticator: {} }
account_repository:
  type: inmem
  common: {}
  inmem: {}
  initial_data:
    users:
      alice:
        uid: 2001
        gid: 2000
        home: "/home/alice"
    groups:
      devs:
        gid: 2000
        home: "/groups/devs"
`
		cfg, err := config.LoadConfigString(yamlStr)
		Expect(err).ToNot(HaveOccurred())

		users := cfg.GetInitialUsers()
		Expect(users).To(HaveLen(1))
		Expect(users["alice"]).ToNot(BeNil())
		Expect(users["alice"].Username).To(Equal("alice"))
		Expect(users["alice"].UID).To(Equal(uint32(2001)))
		Expect(users["alice"].Home).To(Equal("/home/alice"))

		groups := cfg.GetInitialGroups()
		Expect(groups).To(HaveLen(1))
		Expect(groups["devs"]).ToNot(BeNil())
		Expect(groups["devs"].Groupname).To(Equal("devs"))
		Expect(groups["devs"].GID).To(Equal(uint32(2000)))
		Expect(groups["devs"].Home).To(Equal("/groups/devs"))
	})
})

var _ = Describe("DB-related defaults", func() {
	It("applies sqlite timeouts", func() {
		yamlStr := `
storage: { implementation: unix }
http_server: {}
metrics: {}
security: { authenticator: {} }
account_repository:
  type: sqlite
  common: {}
  sqlite:
    db_file_path: "/tmp/test.db"
`
		cfg, err := config.LoadConfigString(yamlStr)
		Expect(err).ToNot(HaveOccurred())

		Expect(cfg.AccountRepository.Sqlite.DbFilePath).To(Equal("/tmp/test.db"))
		Expect(cfg.AccountRepository.Sqlite.QueryTimeout).To(Equal(5 * time.Second))
		Expect(cfg.AccountRepository.Sqlite.WriteTimeout).To(Equal(5 * time.Second))
	})
})

var _ = Describe("ExpandEnvWithDefaults (unit)", func() {
	It("replaces ${VAR} unresolved to empty string if no env and no default", func() {
		_ = os.Unsetenv("NOPE")
		out := config.ExpandEnvWithDefaults("${NOPE}")
		Expect(out).To(Equal(""))
	})

	It("replaces ${VAR:-def} with def when unset", func() {
		_ = os.Unsetenv("NOPE2")
		out := config.ExpandEnvWithDefaults("${NOPE2:-abc}")
		Expect(out).To(Equal("abc"))
	})

	It("replaces ${VAR} with value when set", func() {
		Expect(os.Setenv("REAL", "value")).To(Succeed())
		defer func() {
			_ = os.Unsetenv("REAL")
		}()

		out := config.ExpandEnvWithDefaults("${REAL}")
		Expect(out).To(Equal("value"))
	})
})
