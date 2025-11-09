package api

import (
	"crypto/rand"
	"fmt"
	"fs-access-api/internal/app/ports"
)

func (s *DefaultApiServer) GenerateSecret(requestedSize *int) (size int, secret []byte, err error) {
	size = 32
	if requestedSize != nil {
		if *requestedSize >= 16 && *requestedSize <= 128 {
			size = *requestedSize
		}
	}
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return size, nil, err
	}
	return size, b, nil
}

func (s *DefaultApiServer) ComputeHash(plaintext string, algorithm ports.HashAlgo, rounds *int, saltLen *int) (hash string, err error) {
	// Compute hash
	hash, err = s.hasher.Hash(plaintext, algorithm, rounds, saltLen)
	if err != nil {

		return "", fmt.Errorf("computing hash error: %v", err)
	}
	return hash, nil
}

func (s *DefaultApiServer) VerifyHash(hash, plaintext string) (verified bool, algorithm ports.HashAlgo, err error) {
	return s.hasher.Verify(hash, plaintext)
}
