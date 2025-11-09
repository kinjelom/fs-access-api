package api

import (
	"errors"
	"fs-access-api/internal/app/ports"
)

func (s *DefaultApiServer) ListGroups() ([]ports.GroupInfo, error) {
	return s.accountRepo.ListGroups()
}

func (s *DefaultApiServer) GetGroup(name string) (ports.GroupInfo, error) {
	return s.accountRepo.GetGroup(name)
}

func (s *DefaultApiServer) EnsureGroup(rg ports.GroupInfo) (pg ports.GroupInfo, created bool, err error) {
	pg, err = s.GetGroup(rg.Groupname)
	create := false
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			create = true
		} else {
			return pg, false, err
		}
	}
	if create {
		// Create
		pg, err = s.accountRepo.AddGroup(rg)
		if err != nil {
			return ports.GroupInfo{}, false, err
		}
	} else {
		// Idempotency check
		if !sameGroupData(pg, rg) {
			return ports.GroupInfo{}, false, ports.ErrConflict
		}
	}

	if err = s.fs.PrepareGroupHome(pg); err != nil {
		return ports.GroupInfo{}, false, err
	}
	return pg, create, nil
}

func (s *DefaultApiServer) UpdateGroup(name string, mutate func(obj ports.GroupInfo) (ports.GroupInfo, error)) error {
	pg, err := s.accountRepo.GetGroup(name)
	if err != nil {
		return err
	}
	mg, err := mutate(pg)
	if err != nil {
		return err
	}
	_, err = s.accountRepo.UpdateGroup(mg)
	return err
}

func (s *DefaultApiServer) DeleteGroup(name string) error {
	_, err := s.accountRepo.GetGroup(name)
	if err != nil {
		return ports.ErrNotFound
	}
	err = s.accountRepo.DeleteGroup(name)
	if err != nil {
		return err
	}
	return nil
}

func sameGroupData(a, b ports.GroupInfo) bool {
	if a.Groupname != b.Groupname || a.GID != b.GID || a.Home != b.Home {
		return false
	}
	if (a.Description == nil && b.Description != nil) || (a.Description != nil && b.Description == nil) {
		return false
	}
	if a.Description != nil && b.Description != nil && *a.Description != *b.Description {
		return false
	}

	return true
}
