package api

import (
	"errors"
	"fmt"
	"fs-access-api/internal/app/ports"
)

type AuthzLookupResult struct {
	RootPath string
}

func (s *DefaultApiServer) AuthzLookupUser(username string) (uai *ports.UserAuthzInfo, rootPath string, err error) {
	if username == "" {
		return nil, "", ports.ErrInvalidInput
	}

	uhi, err := s.accountRepo.GetUserAuthzInfo(username)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, "", ports.ErrNotFound
		}
		return nil, "", fmt.Errorf("cannot read user: %w", err)
	}

	if uhi.Locked {
		return nil, "", ports.ErrLockedUser
	}

	return &uhi, s.storageCfg.HomesBaseDir, nil
}

func (s *DefaultApiServer) AuthzAuthUser(username, password string) error {
	if username == "" || password == "" {
		return ports.ErrInvalidInput
	}

	ua, err := s.accountRepo.GetUserAuthzInfo(username)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return ports.ErrInvalidCredentials
		}
		return fmt.Errorf("cannot read user: %w", err)
	}

	if ua.Locked {
		return ports.ErrLockedUser
	}

	ok, _, err := s.hasher.Verify(ua.Password, password)
	if err != nil {
		return fmt.Errorf("password verifier error: %w", err)
	}
	if !ok {
		return ports.ErrInvalidCredentials
	}

	return nil
}
