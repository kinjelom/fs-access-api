package config

import (
	"fmt"
	"fs-access-api/internal/app/ports"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/mcuadros/go-defaults"
	"gopkg.in/yaml.v3"
)

type ProgramConfig struct {
	Storage           StorageConfig           `yaml:"storage"`
	HttpServer        HttpServerConfig        `yaml:"http_server"`
	AccountRepository AccountRepositoryConfig `yaml:"account_repository"`
	Security          SecurityConfig          `yaml:"security"`
	Metrics           MetricsContext          `yaml:"metrics"`
}

type MetricsContext struct {
	Namespace   string `yaml:"namespace" default:"fsaa"`
	Environment string `yaml:"environment"`
}
type StorageConfig struct {
	Implementation     string   `yaml:"implementation" default:"unix"`
	HomesBaseDir       string   `yaml:"homes_base_dir"`
	CreateHomesBaseDir bool     `yaml:"create_homes_base_dir" default:"false"`
	DefaultUserTopDirs []string `yaml:"default_user_top_dirs" default:"[_test]"`
}

type HttpServerConfig struct {
	Banner         string `yaml:"banner" default:"ProFTPD Admin API"`
	ListenAddress  string `yaml:"listen_address" default:":8080"`
	UnixSocketPath string `yaml:"unix_socket_path"`
	TelemetryPath  string `yaml:"telemetry_path" default:"/metrics"`
}

type SecurityConfig struct {
	Authenticator AuthenticatorConfig `yaml:"authenticator"`
	Hasher        HasherConfig        `yaml:"hasher"`
}
type AuthenticatorConfig struct {
	EnabledAuthenticators []string          `yaml:"enabled_authenticators" default:"[hmac,bearer]"`
	WindowSeconds         int               `yaml:"window_seconds" default:"60"`
	AccessKeys            map[string]string `yaml:"access_keys"`
}

type HasherConfig struct {
	DefaultAlgorithm string `yaml:"default_algorithm" default:"crypt-sha256"`
	DefaultRounds    int    `yaml:"default_rounds" default:"5000"`
	DefaultSaltLen   int    `yaml:"default_salt_len" default:"16"`
}

type AccountRepositoryConfig struct {
	Type            string                        `yaml:"type"`
	Common          AccountRepositoryCommonConfig `yaml:"common"`
	LoadInitialData bool                          `yaml:"load_initial_data" default:"false"`
	InitialData     AccountRepositoryInitialData  `yaml:"initial_data"`
	InMem           AccountRepositoryInMemConfig  `yaml:"inmem"`
	Sqlite          AccountRepositorySqliteConfig `yaml:"sqlite"`
	MySQL           AccountRepositoryMySqlConfig  `yaml:"mysql"`
}

type AccountRepositoryCommonConfig struct {
	MinUID uint32 `yaml:"min_uid" default:"2000"`
	MinGID uint32 `yaml:"min_gid" default:"2000"`
}

type AccountRepositoryInitialData struct {
	Users  map[string]ports.UserInfo  `yaml:"users"`
	Groups map[string]ports.GroupInfo `yaml:"groups"`
}

type AccountRepositoryInMemConfig struct {
	EntitiesLimit int `yaml:"entities_limit" default:"1000"`
}

type AccountRepositorySqliteConfig struct {
	DbFilePath   string        `yaml:"db_file_path"`
	CreateDbDir  bool          `yaml:"create_db_dir" default:"false"`
	QueryTimeout time.Duration `yaml:"query_timeout" default:"5s"`
	WriteTimeout time.Duration `yaml:"write_timeout" default:"5s"`
}

type AccountRepositoryMySqlConfig struct {
	Database     string        `yaml:"database"`
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	User         string        `yaml:"user"`
	Password     string        `yaml:"password"`
	IgnoreSSL    bool          `yaml:"ignore_ssl"`
	SSLCaPath    string        `yaml:"ssl_ca_path"`
	QueryTimeout time.Duration `yaml:"query_timeout" default:"5s"`
}

func LoadConfig(path string) (*ProgramConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadConfigString(string(data))
}

func LoadConfigString(data string) (*ProgramConfig, error) {
	expanded := ExpandEnvWithDefaults(data)
	var config ProgramConfig
	err := yaml.Unmarshal([]byte(expanded), &config)
	if err != nil {
		return nil, err
	}
	defaults.SetDefaults(&config)
	return &config, nil
}

func (c *ProgramConfig) PrintHello(programName, programVersion string, pidFile string, bootstrap bool) {
	pid := os.Getpid()
	pidFileInfo := ""
	if pidFile != "" {
		pidFileInfo = " (file: %s)"
	}
	log.Printf("%s v.%s, pid: %d%s, bootstrap: %v, account repository: %s", programName, programVersion, pid, pidFileInfo, bootstrap, c.AccountRepository.Type)
}

func (c *ProgramConfig) GetSecretKey(key string) (string, error) {
	if c.Security.Authenticator.AccessKeys == nil {
		return "", fmt.Errorf("access key %q not found", key)
	}
	if val, ok := c.Security.Authenticator.AccessKeys[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("access key %q not found", key)
}

func (c *ProgramConfig) GetInitialUsers() map[string]*ports.UserInfo {
	out := make(map[string]*ports.UserInfo, len(c.AccountRepository.InitialData.Users))
	if c.AccountRepository.InitialData.Users != nil {
		for name, u := range c.AccountRepository.InitialData.Users {
			uu := u
			uu.Username = name
			out[name] = &uu
		}
	}
	return out
}

func (c *ProgramConfig) GetInitialGroups() map[string]*ports.GroupInfo {
	out := make(map[string]*ports.GroupInfo, len(c.AccountRepository.InitialData.Groups))
	if c.AccountRepository.InitialData.Groups != nil {
		for name, g := range c.AccountRepository.InitialData.Groups {
			gg := g
			gg.Groupname = name
			out[name] = &gg
		}
	}
	return out
}

var varWithDefault = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-(.*?))?}`)

// ExpandEnvWithDefaults handles ${VAR:-default}, ${VAR} and $VAR the env values
func ExpandEnvWithDefaults(s string) string {
	s = varWithDefault.ReplaceAllStringFunc(s, func(m string) string {
		sub := varWithDefault.FindStringSubmatch(m)
		name, defaultVal := sub[1], sub[2]
		if v, ok := os.LookupEnv(name); ok && v != "" {
			return v
		}
		if defaultVal != "" {
			return defaultVal
		}
		// If no default (the pattern was just ${VAR}), keep it unresolved
		return "${" + name + "}"
	})
	// handle $VAR and ${VAR}
	return os.ExpandEnv(s)
}
