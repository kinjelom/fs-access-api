// fs/filesystem.go
//go:build unix

package fs

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	stdos "os"
)

// Conform to your existing port.
var _ ports.FsStorageService = (*DefaultFsStorageService)(nil)

type DefaultFsStorageService struct {
	fs  ports.FilesystemService
	cfg config.StorageConfig
}

func NewDefaultFsStorageService(cfg config.StorageConfig, fs ports.FilesystemService, bootstrap bool) (*DefaultFsStorageService, error) {
	homesBaseDir := filepath.Clean(cfg.HomesBaseDir)
	if bootstrap && cfg.CreateHomesBaseDir {
		if err := fs.MkdirAll(homesBaseDir, 0o777); err != nil {
			return nil, fmt.Errorf("cannot create root directory %q: %w", homesBaseDir, err)
		}
	}
	// Verify homesBaseDir exists and is a directory by attempting ReadDir.
	if _, err := fs.ReadDir(homesBaseDir); err != nil {
		return nil, fmt.Errorf("root directory invalid %q: %w", homesBaseDir, err)
	}
	return &DefaultFsStorageService{fs: fs, cfg: cfg}, nil
}

func (c *DefaultFsStorageService) PrepareGroupHome(group ports.GroupInfo) error {
	groupHome := filepath.Clean(group.Home)
	if strings.HasPrefix(groupHome, string(filepath.Separator)) {
		return fmt.Errorf("cannot prepare group home using absolute path: %q", groupHome)
	}
	absGroupHome := filepath.Clean(filepath.Join(c.cfg.HomesBaseDir, groupHome))
	if !strings.HasPrefix(absGroupHome+string(filepath.Separator), c.cfg.HomesBaseDir+string(filepath.Separator)) {
		return fmt.Errorf("group home %q escapes root %q", absGroupHome, c.cfg.HomesBaseDir)
	}
	return ensureDir(c.fs, absGroupHome, 0o751, 0, group.GID, false)
}

func (c *DefaultFsStorageService) PrepareUserHome(user ports.UserInfo, group ports.GroupInfo) error {
	groupHome := filepath.Clean(group.Home)
	if strings.HasPrefix(groupHome, string(filepath.Separator)) {
		return fmt.Errorf("cannot prepare group home using absolute path: %q", groupHome)
	}
	userHome := filepath.Clean(user.Home)
	if strings.HasPrefix(userHome, string(filepath.Separator)) {
		return fmt.Errorf("cannot prepare user home using absolute path: %q", userHome)
	}
	absGroupHome := filepath.Clean(filepath.Join(c.cfg.HomesBaseDir, groupHome))
	absUserHome := filepath.Clean(filepath.Join(absGroupHome, userHome))
	if !strings.HasPrefix(absUserHome+string(filepath.Separator), absGroupHome+string(filepath.Separator)) {
		return fmt.Errorf("user home %q escapes group %q", absUserHome, absGroupHome)
	}
	if err := ensureDir(c.fs, absUserHome, 0o751, user.UID, group.GID, false); err != nil {
		return err
	}
	for _, topDir := range c.cfg.DefaultUserTopDirs {
		err := ensureDir(c.fs, filepath.Join(absUserHome, topDir), 0o2770, user.UID, group.GID, true)
		if err != nil {
			return fmt.Errorf("cannot create user '%s' top dir '%s': %w", userHome, topDir, err)
		}
	}
	return nil
}

func (c *DefaultFsStorageService) CreateUserTopDir(user ports.UserInfo, group ports.GroupInfo, topDir string) error {
	groupHome := filepath.Clean(group.Home)
	if strings.HasPrefix(groupHome, string(filepath.Separator)) {
		return fmt.Errorf("cannot prepare group home using absolute path: %q", groupHome)
	}
	userHome := filepath.Clean(user.Home)
	if strings.HasPrefix(userHome, string(filepath.Separator)) {
		return fmt.Errorf("cannot prepare user home using absolute path: %q", userHome)
	}
	topDir = filepath.Clean(topDir)
	if strings.HasPrefix(topDir, string(filepath.Separator)) {
		return fmt.Errorf("cannot prepare top dir using absolute path: %q", topDir)
	}

	absUserHome := filepath.Clean(filepath.Join(c.cfg.HomesBaseDir, groupHome, userHome))
	absTop := filepath.Clean(filepath.Join(absUserHome, topDir))
	if !strings.HasPrefix(absTop+string(filepath.Separator), absUserHome+string(filepath.Separator)) {
		return fmt.Errorf("top dir %q escapes user home %q", absTop, absUserHome)
	}
	// enforce “top-level” (no nested paths)
	if filepath.Dir(absTop) != absUserHome {
		return fmt.Errorf("refusing non-top-level directory: %q", absTop)
	}
	return ensureDir(c.fs, absTop, 0o2770, user.UID, group.GID, true)
}

func (c *DefaultFsStorageService) ListUserTopDirs(user ports.UserInfo, group ports.GroupInfo) ([]string, error) {
	groupHome := filepath.Clean(group.Home)
	if strings.HasPrefix(groupHome, string(filepath.Separator)) {
		return nil, fmt.Errorf("cannot list: absolute group home: %q", groupHome)
	}
	userHome := filepath.Clean(user.Home)
	if strings.HasPrefix(userHome, string(filepath.Separator)) {
		return nil, fmt.Errorf("cannot list: absolute user home: %q", userHome)
	}

	absGroupHome := filepath.Clean(filepath.Join(c.cfg.HomesBaseDir, groupHome))
	absUserHome := filepath.Clean(filepath.Join(absGroupHome, userHome))
	if !strings.HasPrefix(absUserHome+string(filepath.Separator), absGroupHome+string(filepath.Separator)) {
		return nil, fmt.Errorf("user home %q escapes group %q", absUserHome, absGroupHome)
	}

	entries, err := c.fs.ReadDir(absUserHome) // succeeds only for real directories
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, e := range entries {
		// ignore symlinks: ReadDir’s DirEntry.Type doesn’t follow symlinks
		if e.Type()&stdos.ModeSymlink != 0 {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

func (c *DefaultFsStorageService) DeleteUserTopDir(user ports.UserInfo, group ports.GroupInfo, topDir string) error {
	groupHome := filepath.Clean(group.Home)
	if strings.HasPrefix(groupHome, string(filepath.Separator)) {
		return fmt.Errorf("cannot delete: absolute group home: %q", groupHome)
	}
	userHome := filepath.Clean(user.Home)
	if strings.HasPrefix(userHome, string(filepath.Separator)) {
		return fmt.Errorf("cannot delete: absolute user home: %q", userHome)
	}
	topDir = filepath.Clean(topDir)
	if strings.HasPrefix(topDir, string(filepath.Separator)) {
		return fmt.Errorf("cannot delete: absolute top dir: %q", topDir)
	}

	absGroupHome := filepath.Clean(filepath.Join(c.cfg.HomesBaseDir, groupHome))
	absUserHome := filepath.Clean(filepath.Join(absGroupHome, userHome))
	if !strings.HasPrefix(absUserHome+string(filepath.Separator), absGroupHome+string(filepath.Separator)) {
		return fmt.Errorf("user home %q escapes group %q", absUserHome, absGroupHome)
	}

	absTop := filepath.Clean(filepath.Join(absUserHome, topDir))
	if !strings.HasPrefix(absTop+string(filepath.Separator), absUserHome+string(filepath.Separator)) {
		return fmt.Errorf("top dir %q escapes user home %q", absTop, absUserHome)
	}
	if filepath.Dir(absTop) != absUserHome {
		return fmt.Errorf("refusing non-top-level directory: %q", absTop)
	}

	// Confirm it is a directory (ReadDir works only on directories)
	if _, err := c.fs.ReadDir(absTop); err != nil {
		// if not exists or not a dir -> error out similarly to before
		if errors.Is(err, stdos.ErrNotExist) {
			return fmt.Errorf("top dir does not exist: %q", absTop)
		}
		return fmt.Errorf("cannot open top dir %q: %w", absTop, err)
	}
	return c.fs.RemoveAll(absTop)
}

/* ---------- 4) Single helper for all dir creation cases ---------- */

func ensureDir(fsys ports.FilesystemService, path string, mode fs.FileMode, uid, gid uint32, setgid bool) error {
	if err := fsys.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	if err := fsys.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", path, err)
	}
	if setgid {
		mode |= 0o2000 // setgid bit
	}
	// force exact perms (bypass umask effects)
	if err := fsys.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}
