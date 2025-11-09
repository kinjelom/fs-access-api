package ports

type ApiServer interface {
	HealthCheck() error
	AuthzLookupUser(username string) (uai *UserAuthzInfo, baseDir string, err error)
	AuthzAuthUser(username, password string) (err error)
	GenerateSecret(requestedSize *int) (size int, secret []byte, err error)
	ComputeHash(plaintext string, algorithm HashAlgo, rounds *int, saltLen *int) (hash string, err error)
	VerifyHash(hash, plaintext string) (verified bool, algorithm HashAlgo, err error)

	ListGroups() ([]GroupInfo, error)
	GetGroup(name string) (GroupInfo, error)
	EnsureGroup(group GroupInfo) (gi GroupInfo, created bool, err error)
	UpdateGroup(name string, mutate func(group GroupInfo) (GroupInfo, error)) error
	DeleteGroup(name string) error

	ListUsers() ([]UserInfo, error)
	GetUser(name string) (UserInfo, error)
	EnsureUser(user UserInfo) (ui UserInfo, created bool, err error)
	UpdateUser(name string, mutate func(user UserInfo) (UserInfo, error)) error
	DeleteUser(name string) error

	ListUserDirs(username string) (dirs []string, err error)
	DeleteUserDir(username string, dirname string) error
	EnsureUserDir(username string, dirname string) (created bool, err error)
}
