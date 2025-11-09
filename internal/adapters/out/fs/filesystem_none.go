package fs

import (
	"fs-access-api/internal/app/ports"
	"io/fs"
)

type NoneFilesystemService struct{}

var _ ports.FilesystemService = (*NoneFilesystemService)(nil)

func NewNoneFilesystemService() *NoneFilesystemService {
	return &NoneFilesystemService{}
}

func (NoneFilesystemService) GetInfo(_ string) (fi fs.FileInfo, uid, gid uint32, err error) {
	return nil, 0, 0, err

}
func (NoneFilesystemService) Mkdir(_ string, _ fs.FileMode) error {
	return nil
}
func (NoneFilesystemService) MkdirAll(_ string, _ fs.FileMode) error {
	return nil
}
func (NoneFilesystemService) Chown(_ string, _, _ uint32) error {
	return nil
}
func (NoneFilesystemService) Chmod(_ string, _ fs.FileMode) error     { return nil }
func (NoneFilesystemService) ReadDir(_ string) ([]fs.DirEntry, error) { return []fs.DirEntry{}, nil }
func (NoneFilesystemService) Remove(_ string) error                   { return nil }
func (NoneFilesystemService) RemoveAll(_ string) error                { return nil }
