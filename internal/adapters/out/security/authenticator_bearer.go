package security

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"net/http"
	"strings"
)

type BearerAuthenticator struct {
	// accessSecrets maps public key-id -> secret bytes
	accessSecrets map[string]string
}

// Enforce compile-time conformance to the interface
var _ ports.Authenticator = (*BearerAuthenticator)(nil)

// Headers as constants for consistency
const (
	bearerScheme = "Bearer"
)

func NewBearerAuthenticator(authCfg config.AuthenticatorConfig) (*BearerAuthenticator, error) {
	// decode hex secrets
	secrets := make(map[string]string, len(authCfg.AccessKeys))
	for keyID, hexSecret := range authCfg.AccessKeys {
		hexSecret = strings.TrimSpace(hexSecret)
		if hexSecret == "" {
			return nil, errors.New("empty secret for key " + keyID)
		}
		_, err := hex.DecodeString(hexSecret)
		if err != nil {
			return nil, errors.New("invalid hex secret for key " + keyID + ": " + err.Error())
		}
		secrets[keyID] = hexSecret
	}

	return &BearerAuthenticator{
		accessSecrets: secrets,
	}, nil
}

func (s *BearerAuthenticator) Supports(r *http.Request) bool {
	authz := r.Header.Get(hdrAuthz)
	return strings.HasPrefix(authz, bearerScheme+" ")
}

// Verify does pure auth logic; no writes to ResponseWriter.
func (s *BearerAuthenticator) Verify(r *http.Request) error {
	apiKey := r.Header.Get(hdrAPIKey)
	authz := r.Header.Get(hdrAuthz)

	if apiKey == "" || authz == "" {
		return fmt.Errorf("missing auth headers")
	}
	secretHex, ok := s.accessSecrets[apiKey]
	if !ok {
		return fmt.Errorf("unknown api key")
	}
	if !strings.HasPrefix(authz, bearerScheme+" ") {
		return fmt.Errorf("invalid auth scheme")
	}
	sigHex := strings.TrimPrefix(authz, bearerScheme+" ")
	if sigHex != secretHex {
		return fmt.Errorf("not verified")
	}
	return nil
}

func (s *BearerAuthenticator) WithAuthChi(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.Verify(r); err != nil {
			// map reasons to 401/400; keep it terse
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Optionally inject principal (e.g., apiKey) into context
		ctx := context.WithValue(r.Context(), ctxKeyPrincipal, r.Header.Get(hdrAPIKey))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
