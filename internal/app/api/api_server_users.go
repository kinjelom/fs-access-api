package api

import (
	"errors"
	"fs-access-api/internal/app/ports"
)

func (s *DefaultApiServer) ListUsers() ([]ports.UserInfo, error) {
	return s.accountRepo.ListUsers()
}

func (s *DefaultApiServer) GetUser(username string) (ports.UserInfo, error) {
	return s.accountRepo.GetUser(username)
}

func (s *DefaultApiServer) EnsureUser(ru ports.UserInfo) (pu ports.UserInfo, created bool, err error) {
	create := false
	pu, err = s.GetUser(ru.Username)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			create = true
		} else {
			return pu, false, err
		}
	}
	if create {
		// Create
		if ru.UID == 0 {
			var uid uint32
			uid, err = s.accountRepo.GetNextUID()
			if err != nil {
				return ports.UserInfo{}, false, err
			}
			ru.UID = uid
		}
		var hash string
		hash, err = s.preparePassword(ru.Password, ru.PasswordIsHash)
		if err != nil {
			return ports.UserInfo{}, false, err
		}
		ru.Password = hash
		ru.PasswordIsHash = true

		pu, err = s.accountRepo.AddUser(ru)
		if err != nil {
			return ports.UserInfo{}, false, err
		}
	} else {
		// Idempotency check
		ru.UID = pu.UID
		// User exists: verify idempotency (all fields equal AND password matches stored hash)
		if !s.sameUserData(pu, ru, ru.PasswordIsHash) {
			return ports.UserInfo{}, false, ports.ErrConflict
		}
	}

	group, err := s.accountRepo.GetGroup(ru.Groupname)
	if err != nil {
		return ports.UserInfo{}, false, err
	}

	if err = s.fs.PrepareUserHome(pu, group); err != nil {
		return ports.UserInfo{}, false, err
	}
	return pu, create, nil
}

func (s *DefaultApiServer) UpdateUser(username string, mutate func(obj ports.UserInfo) (ports.UserInfo, error)) error {
	pg, err := s.accountRepo.GetUser(username)
	if err != nil {
		return err
	}
	mg, err := mutate(pg)
	if err != nil {
		return err
	}
	hash, err := s.preparePassword(mg.Password, mg.PasswordIsHash)
	if err != nil {
		return err
	}
	mg.Password = hash
	mg.PasswordIsHash = true

	_, err = s.accountRepo.UpdateUser(mg)
	return err
}

func (s *DefaultApiServer) DeleteUser(username string) error {
	_, err := s.accountRepo.GetUser(username)
	if err != nil {
		return err
	}
	err = s.accountRepo.DeleteUser(username)
	if err != nil {
		return err
	}
	return nil
}

func (s *DefaultApiServer) ListUserDirs(username string) (dirs []string, err error) {
	fu, err := s.accountRepo.GetUser(username)
	if err != nil {
		return []string{}, err
	}
	fg, err := s.accountRepo.GetGroup(fu.Groupname)
	if err != nil {
		return []string{}, err
	}
	return s.fs.ListUserTopDirs(fu, fg)
}

func (s *DefaultApiServer) DeleteUserDir(username string, dirname string) error {
	fu, err := s.accountRepo.GetUser(username)
	if err != nil {
		return err
	}
	fg, err := s.accountRepo.GetGroup(fu.Groupname)
	if err != nil {
		return err
	}
	return s.fs.DeleteUserTopDir(fu, fg, dirname)
}

func (s *DefaultApiServer) EnsureUserDir(username string, dirname string) (created bool, err error) {
	fu, err := s.accountRepo.GetUser(username)
	if err != nil {
		return false, err
	}
	fg, err := s.accountRepo.GetGroup(fu.Groupname)
	if err != nil {
		return false, err
	}
	dirs, err := s.fs.ListUserTopDirs(fu, fg)
	if err != nil {
		return false, err
	}
	exists := false
	if dirs != nil {
		for _, dir := range dirs {
			if dir == dirname {
				exists = true
				break
			}
		}
	}
	err = s.fs.CreateUserTopDir(fu, fg, dirname)
	return !exists && err == nil, err
}

func (s *DefaultApiServer) sameUserData(up, ur ports.UserInfo, reqPasswordIsHashed bool) bool {
	if up.Username != ur.Username || up.Groupname != ur.Groupname || up.Home != ur.Home || up.Disabled != ur.Disabled {
		return false
	}

	if (up.Expiration == nil && ur.Expiration != nil) || (up.Expiration != nil && ur.Expiration == nil) {
		return false
	}
	if up.Expiration != nil && ur.Expiration != nil && !(*up.Expiration).Equal(*ur.Expiration) {
		return false
	}

	if (up.Description == nil && ur.Description != nil) || (up.Description != nil && ur.Description == nil) {
		return false
	}
	if up.Description != nil && ur.Description != nil && *up.Description != *ur.Description {
		return false
	}

	if reqPasswordIsHashed {
		if up.Password != ur.Password {
			return false
		}
	} else {
		verified, _, _ := s.hasher.Verify(up.Password, ur.Password)
		if !verified {
			return false
		}
	}
	return true
}

func (s *DefaultApiServer) preparePassword(password string, passwordIsHash bool) (string, error) {
	// Password - handle both plain and hashed values
	if password == "" {
		return "", errors.New("password is required")
	}
	if passwordIsHash {
		return password, nil
	} else {
		return s.hasher.DefaultHash(password)
	}
}
