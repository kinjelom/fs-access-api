package api

import (
	"errors"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
)

// Enforce compile-time conformance to a generated interface
var _ ports.ApiServer = (*DefaultApiServer)(nil)

type DefaultApiServer struct {
	storageCfg  config.StorageConfig
	hasher      ports.Hasher
	accountRepo ports.AccountRepository
	fs          ports.FsStorageService
}

func NewDefaultApiServer(cfg config.StorageConfig, hasher ports.Hasher, accountRepo ports.AccountRepository, fs ports.FsStorageService) (*DefaultApiServer, error) {
	if accountRepo == nil {
		return nil, errors.New("accountRepo is nil")
	}
	if fs == nil {
		return nil, errors.New("file system service is nil")
	}
	return &DefaultApiServer{
		storageCfg:  cfg,
		hasher:      hasher,
		accountRepo: accountRepo,
		fs:          fs,
	}, nil
}

func (s *DefaultApiServer) HealthCheck() error {
	return s.accountRepo.HealthCheck()
}
