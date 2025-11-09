package ports

import (
	"io/fs"
)

type FilesystemService interface {
	GetInfo(path string) (fi fs.FileInfo, uid, gid uint32, err error)
	Mkdir(path string, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Chown(path string, uid, gid uint32) error
	Chmod(path string, perm fs.FileMode) error
	ReadDir(path string) ([]fs.DirEntry, error)
	Remove(path string) error
	RemoveAll(path string) error
}
