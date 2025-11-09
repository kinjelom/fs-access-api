package ports

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrAlreadyExists = errors.New("already exists")

	ErrInvalidInput       = errors.New("invalid input")
	ErrLockedUser         = errors.New("user is locked")
	ErrInvalidCredentials = errors.New("invalid credentials")

	ErrUnsupportedAlgorithm = errors.New("unsupported algorithm")
	ErrUnsupportedAction    = errors.New("unsupported action")
)
