package fs

import (
	"fmt"
	"fs-access-api/internal/app/ports"
	"io/fs"
	"os"
	"syscall"
)

type UnixFilesystemService struct{}

var _ ports.FilesystemService = (*UnixFilesystemService)(nil)

func NewUnixFilesystemService() *UnixFilesystemService {
	return &UnixFilesystemService{}
}

func (UnixFilesystemService) GetInfo(p string) (fi fs.FileInfo, uid, gid uint32, err error) {
	fi, err = os.Lstat(p)
	if err != nil {
		return nil, 0, 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, 0, 0, fmt.Errorf("stat_t not available for %s", p)
	}
	return fi, st.Uid, st.Gid, nil
}
func (UnixFilesystemService) Mkdir(p string, perm fs.FileMode) error {
	return os.Mkdir(p, perm)
}
func (UnixFilesystemService) MkdirAll(p string, perm fs.FileMode) error {
	return os.MkdirAll(p, perm)
}
func (UnixFilesystemService) Chown(p string, uid, gid uint32) error {
	return os.Chown(p, int(uid), int(gid))
}
func (UnixFilesystemService) Chmod(p string, perm fs.FileMode) error  { return os.Chmod(p, perm) }
func (UnixFilesystemService) ReadDir(p string) ([]fs.DirEntry, error) { return os.ReadDir(p) }
func (UnixFilesystemService) Remove(p string) error                   { return os.Remove(p) }
func (UnixFilesystemService) RemoveAll(p string) error                { return os.RemoveAll(p) }
