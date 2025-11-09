package ports

import (
	"path/filepath"
	"time"
)

type AccountRepository interface {
	HealthCheck() error
	GetInfo() (string, error)

	ListGroups() ([]GroupInfo, error)
	GetGroup(name string) (GroupInfo, error)
	AddGroup(group GroupInfo) (GroupInfo, error)
	UpdateGroup(group GroupInfo) (GroupInfo, error)
	DeleteGroup(name string) error

	GetNextUID() (uint32, error)
	ListUsers() ([]UserInfo, error)
	GetUser(name string) (UserInfo, error)
	AddUser(user UserInfo) (UserInfo, error)
	UpdateUser(user UserInfo) (UserInfo, error)
	DeleteUser(name string) error

	GetUserAuthzInfo(name string) (UserAuthzInfo, error)
}

type GroupInfo struct {
	Groupname   string  `yaml:"groupname"`
	GID         uint32  `yaml:"gid"`
	Description *string `yaml:"description" json:"description,omitempty"`
	Home        string  `yaml:"home"  json:"home"`
}

func (g *GroupInfo) AbsoluteHomeDir(homesBaseDir string) string {
	return filepath.Clean(filepath.Join(homesBaseDir, g.Home))
}

type UserInfo struct {
	Username       string     `yaml:"username" json:"username"`
	UID            uint32     `yaml:"uid"   json:"uid"`
	Groupname      string     `yaml:"groupname" json:"groupname"`
	Password       string     `yaml:"password" json:"-"`
	PasswordIsHash bool       `yaml:"password_is_hash" json:"-"`
	Description    *string    `yaml:"description" json:"description,omitempty"`
	Home           string     `yaml:"home"  json:"home"`
	Expiration     *time.Time `yaml:"expiration,omitempty" json:"expiration,omitempty"`
	Disabled       bool       `yaml:"disabled" json:"disabled"`
}

func IsUserLocked(disabled bool, expiration *time.Time) bool {
	return disabled || (expiration != nil && expiration.Before(time.Now()))
}

func (u *UserInfo) IsLocked() bool {
	return IsUserLocked(u.Disabled, u.Expiration)
}

func (u *UserInfo) AbsoluteHomeDir(homesBaseDir, groupHome string) string {
	return filepath.Clean(filepath.Join(homesBaseDir, groupHome, u.Home))
}

type UserAuthzInfo struct {
	Username  string `yaml:"username" json:"username"`
	UID       uint32 `yaml:"uid"   json:"uid"`
	Groupname string `yaml:"groupname" json:"groupname"`
	GID       uint32 `yaml:"gid"`
	UserHome  string `yaml:"user-home"  json:"user-home"`
	GroupHome string `yaml:"group-home"  json:"group-home"`
	Locked    bool   `yaml:"locked" json:"locked"`
	Password  string `yaml:"password" json:"-"`
}

func (u *UserAuthzInfo) AbsoluteHomeDir(homesBaseDir string) string {
	return filepath.Clean(filepath.Join(homesBaseDir, u.GroupHome, u.UserHome))
}
