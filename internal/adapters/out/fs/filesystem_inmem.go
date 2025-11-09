package fs

import (
	"errors"
	"fmt"
	"fs-access-api/internal/app/ports"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// InMemFilesystemService is a simple in-memory directory tree
// implementing FilesystemService (for tests and unit logic).
type InMemFilesystemService struct {
	root *memDir
}

var _ ports.FilesystemService = (*InMemFilesystemService)(nil)

type memDir struct {
	name string
	mode fs.FileMode
	uid  uint32
	gid  uint32
	sub  map[string]*memDir
}

func NewInMemFilesystemService() *InMemFilesystemService {
	return &InMemFilesystemService{
		root: &memDir{
			name: "/",
			mode: 0o755,
			sub:  map[string]*memDir{},
		},
	}
}

func (m *InMemFilesystemService) GetInfo(p string) (fi fs.FileInfo, uid, gid uint32, err error) {
	if p == "" || p == "/" || p == "." {
		return nil, 0, 0, nil
	}
	d, err := m.lookupDir(p, true)
	if err != nil {
		return nil, 0, 0, err
	}
	return memFileInfo{d}, d.uid, d.gid, nil
}

func (m *InMemFilesystemService) Mkdir(p string, perm fs.FileMode) error {
	if p == "" || p == "/" || p == "." {
		return fmt.Errorf("invalid directory path: %q", p)
	}

	parts := splitPath(p)
	if len(parts) == 0 {
		return fmt.Errorf("invalid path: %q", p)
	}

	parentParts := parts[:len(parts)-1]
	name := parts[len(parts)-1]

	parent, err := m.lookupDir(joinPath(parentParts), false)
	if err != nil {
		return fmt.Errorf("parent directory not found: %w", err)
	}

	if _, ok := parent.sub[name]; ok {
		return fs.ErrExist
	}

	m.createDir(parent, name, perm)
	return nil
}

func (m *InMemFilesystemService) MkdirAll(p string, perm fs.FileMode) error {
	if p == "" || p == "/" || p == "." {
		return nil
	}
	d, err := m.lookupDir(p, true)
	if err != nil {
		return err
	}
	d.mode = perm
	return nil
}

func (m *InMemFilesystemService) Chown(p string, uid, gid uint32) error {
	d, err := m.lookupDir(p, false)
	if err != nil {
		return err
	}
	d.uid, d.gid = uid, gid
	return nil
}

func (m *InMemFilesystemService) Chmod(p string, perm fs.FileMode) error {
	d, err := m.lookupDir(p, false)
	if err != nil {
		return err
	}
	d.mode = perm
	return nil
}

func (m *InMemFilesystemService) ReadDir(p string) ([]fs.DirEntry, error) {
	d, err := m.lookupDir(p, false)
	if err != nil {
		return nil, fmt.Errorf("not a directory: %w", err)
	}
	names := make([]string, 0, len(d.sub))
	for k := range d.sub {
		names = append(names, k)
	}
	sort.Strings(names)

	out := make([]fs.DirEntry, 0, len(names))
	for _, n := range names {
		out = append(out, memDirEntry{d.sub[n]})
	}
	return out, nil
}

func (m *InMemFilesystemService) Remove(p string) error {
	if p == "" || p == "/" || p == "." {
		return errors.New("refusing to remove root or invalid path")
	}

	parts := splitPath(p)
	if len(parts) == 0 {
		return errors.New("invalid path")
	}

	base := parts[len(parts)-1]
	parentParts := parts[:len(parts)-1]

	parent, err := m.lookupDir(filepath.Join(parentParts...), false)
	if err != nil {
		return fmt.Errorf("parent not found: %w", err)
	}

	target, ok := parent.sub[base]
	if !ok {
		return fs.ErrNotExist
	}

	if len(target.sub) > 0 {
		return fmt.Errorf("directory not empty: %s", p)
	}

	delete(parent.sub, base)
	return nil
}

func (m *InMemFilesystemService) RemoveAll(p string) error {
	if p == "/" || p == "." {
		return errors.New("refusing to remove root")
	}
	parts := splitPath(p)
	if len(parts) == 0 {
		return errors.New("invalid path")
	}
	base := parts[len(parts)-1]
	parentParts := parts[:len(parts)-1]

	parent, err := m.lookupDir(filepath.Join(parentParts...), false)
	if err != nil {
		return err
	}
	delete(parent.sub, base)
	return nil
}

/* ---------- Helpers ---------- */

func splitPath(p string) []string {
	p = filepath.Clean(p)
	if p == "/" || p == "." {
		return nil
	}
	if strings.HasPrefix(p, string(filepath.Separator)) {
		p = strings.TrimPrefix(p, string(filepath.Separator))
	}
	if p == "" {
		return nil
	}
	return strings.Split(p, string(filepath.Separator))
}

func joinPath(parts []string) string {
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}

func (m *InMemFilesystemService) lookupDir(p string, create bool) (*memDir, error) {
	parts := splitPath(p)

	cur := m.root
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}

		next, ok := cur.sub[part]
		if !ok {
			if !create {
				return nil, fs.ErrNotExist
			}
			next = m.createDir(cur, part, 0o755)
		}
		cur = next
	}
	return cur, nil
}

func (m *InMemFilesystemService) createDir(parent *memDir, name string, mode fs.FileMode) *memDir {
	d := &memDir{
		name: name,
		mode: mode,
		sub:  make(map[string]*memDir),
	}
	parent.sub[name] = d
	return d
}

/* ---------- DirEntry wrapper ---------- */
type memDirEntry struct {
	d *memDir
}

func (e memDirEntry) Name() string               { return e.d.name }
func (e memDirEntry) IsDir() bool                { return true }
func (e memDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e memDirEntry) Info() (fs.FileInfo, error) { return memFileInfo{e.d}, nil }

/* ---------- FileInfo wrapper ---------- */

type memFileInfo struct {
	d *memDir
}

var _ fs.FileInfo = (*memFileInfo)(nil)

func (f memFileInfo) Name() string       { return f.d.name }
func (f memFileInfo) Size() int64        { return 0 }
func (f memFileInfo) Mode() fs.FileMode  { return f.d.mode | fs.ModeDir }
func (f memFileInfo) ModTime() time.Time { return time.Time{} }
func (f memFileInfo) IsDir() bool        { return true }
func (f memFileInfo) Sys() any           { return nil }
