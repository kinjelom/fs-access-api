package ports

type FsStorageService interface {
	PrepareGroupHome(group GroupInfo) error
	PrepareUserHome(user UserInfo, group GroupInfo) error
	CreateUserTopDir(user UserInfo, group GroupInfo, topDir string) error
	ListUserTopDirs(user UserInfo, group GroupInfo) ([]string, error)
	DeleteUserTopDir(user UserInfo, group GroupInfo, topDir string) error
}
