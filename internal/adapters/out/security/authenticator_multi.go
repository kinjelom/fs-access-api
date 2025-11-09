package security

import (
	"context"
	"fmt"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"net/http"
)

const (
	ctxKeyPrincipal ctxKey = "principal"
	hdrAPIKey              = "X-Api-Key"
	hdrAuthz               = "Authorization"
)

type MultiAuthenticator struct {
	authenticators map[string]ports.Authenticator
}

// Enforce compile-time conformance to the interface
var _ ports.Authenticator = (*MultiAuthenticator)(nil)

func NewMultiAuthenticator(authCfg config.AuthenticatorConfig) (*MultiAuthenticator, error) {
	authenticators := make(map[string]ports.Authenticator, len(authCfg.EnabledAuthenticators))
	for _, authenticatorName := range authCfg.EnabledAuthenticators {
		if authenticatorName == "hmac" {
			authenticator, err := NewHMACAuthenticator(authCfg)
			if err != nil {
				return nil, fmt.Errorf("can't create HMAC authenticator: %w", err)
			}
			authenticators[authenticatorName] = authenticator
		} else if authenticatorName == "bearer" {
			authenticator, err := NewBearerAuthenticator(authCfg)
			if err != nil {
				return nil, fmt.Errorf("can't create Bearer authenticator: %w", err)
			}
			authenticators[authenticatorName] = authenticator
		}
	}
	return &MultiAuthenticator{authenticators: authenticators}, nil
}

func (s *MultiAuthenticator) Supports(r *http.Request) bool {
	for _, authenticator := range s.authenticators {
		if authenticator.Supports(r) {
			return true
		}
	}
	return false
}

// Verify does pure auth logic; no writes to ResponseWriter.
func (s *MultiAuthenticator) Verify(r *http.Request) error {
	authz := r.Header.Get(hdrAuthz)
	if authz == "" {
		return fmt.Errorf("missing '" + hdrAuthz + "' header")
	}
	for _, authenticator := range s.authenticators {
		if authenticator.Supports(r) {
			return authenticator.Verify(r)
		}
	}
	return fmt.Errorf("authorization scheme not supported")
}

func (s *MultiAuthenticator) WithAuthChi(next http.Handler) http.Handler {
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
