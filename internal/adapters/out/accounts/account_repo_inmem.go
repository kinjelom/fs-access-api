package accounts

import (
	"errors"
	"fmt"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"sync"
)

type InMemAccountRepository struct {
	cfg       config.AccountRepositoryInMemConfig
	common    config.AccountRepositoryCommonConfig
	bootstrap bool
	users     map[string]*ports.UserInfo
	groups    map[string]*ports.GroupInfo
	mu        sync.RWMutex
}

// Enforce compile-time conformance to the interface
var _ ports.AccountRepository = (*InMemAccountRepository)(nil)

func NewInMemAccountRepository(cfg config.AccountRepositoryInMemConfig, common config.AccountRepositoryCommonConfig, bootstrap bool) (*InMemAccountRepository, error) {
	return &InMemAccountRepository{
		cfg:       cfg,
		common:    common,
		bootstrap: bootstrap,
		users:     make(map[string]*ports.UserInfo),
		groups:    make(map[string]*ports.GroupInfo),
	}, nil
}

func (s *InMemAccountRepository) HealthCheck() error {
	return nil
}

func (s *InMemAccountRepository) GetInfo() (string, error) {
	return "in-memory", nil
}

// --- Groups ---

func (s *InMemAccountRepository) ListGroups() ([]ports.GroupInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ports.GroupInfo, 0, len(s.groups))
	for _, g := range s.groups {
		out = append(out, *g)
	}
	return out, nil
}

func (s *InMemAccountRepository) GetGroup(name string) (ports.GroupInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.groups[name]
	if !ok {
		return ports.GroupInfo{}, ports.ErrNotFound
	}
	return *g, nil
}

func (s *InMemAccountRepository) AddGroup(group ports.GroupInfo) (ports.GroupInfo, error) {
	if len(s.groups) >= s.cfg.EntitiesLimit {
		return ports.GroupInfo{}, fmt.Errorf("groups limit reached")
	}
	if group.GID < s.common.MinGID {
		return ports.GroupInfo{}, fmt.Errorf("group GID is lower than %d", s.common.MinGID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if group.Groupname == "" {
		return ports.GroupInfo{}, errors.New("group name is required")
	}
	if _, exists := s.groups[group.Groupname]; exists {
		return ports.GroupInfo{}, ports.ErrAlreadyExists
	}
	g := group
	s.groups[group.Groupname] = &g
	return group, nil
}

func (s *InMemAccountRepository) UpdateGroup(group ports.GroupInfo) (ports.GroupInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ptr, exists := s.groups[group.Groupname]
	if !exists {
		return ports.GroupInfo{}, ports.ErrNotFound
	}
	*ptr = group
	return group, nil
}

func (s *InMemAccountRepository) DeleteGroup(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.groups[name]
	if !exists {
		return ports.ErrNotFound
	}
	delete(s.groups, name)
	return nil
}

// --- Users ---

func (s *InMemAccountRepository) ListUsers() ([]ports.UserInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ports.UserInfo, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, *u) // return values to callers to avoid external mutation
	}
	return out, nil
}

func (s *InMemAccountRepository) GetUser(name string) (ports.UserInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[name]
	if !ok {
		return ports.UserInfo{}, ports.ErrNotFound
	}
	return *u, nil
}

func (s *InMemAccountRepository) GetNextUID() (uint32, error) {
	return s.common.MinUID + uint32(len(s.users)), nil
}

func (s *InMemAccountRepository) AddUser(user ports.UserInfo) (ports.UserInfo, error) {
	if len(s.users) >= s.cfg.EntitiesLimit {
		return ports.UserInfo{}, fmt.Errorf("users limit reached")
	}
	if user.UID < s.common.MinUID {
		return ports.UserInfo{}, fmt.Errorf("user UID is lower than %d", s.common.MinGID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if user.Username == "" {
		return ports.UserInfo{}, errors.New("user name is required")
	}
	if _, exists := s.users[user.Username]; exists {
		return ports.UserInfo{}, ports.ErrAlreadyExists
	}
	u := user
	s.users[user.Username] = &u
	return u, nil
}

func (s *InMemAccountRepository) UpdateUser(user ports.UserInfo) (ports.UserInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.users[user.Username]
	if !ok {
		return ports.UserInfo{}, ports.ErrNotFound
	}

	*existing = user
	return *existing, nil
}

func (s *InMemAccountRepository) DeleteUser(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[name]; !exists {
		return ports.ErrNotFound
	}
	delete(s.users, name)
	return nil
}

func (s *InMemAccountRepository) GetUserAuthzInfo(username string) (ports.UserAuthzInfo, error) {
	u, err := s.GetUser(username)
	if err != nil {
		return ports.UserAuthzInfo{}, err
	}
	g, err := s.GetGroup(u.Groupname)
	if err != nil {
		return ports.UserAuthzInfo{}, err
	}
	return ports.UserAuthzInfo{
		Username:  u.Username,
		UID:       u.UID,
		Groupname: u.Groupname,
		GID:       g.GID,
		UserHome:  u.Home,
		GroupHome: g.Home,
		Locked:    u.IsLocked(),
		Password:  u.Password,
	}, nil
}
