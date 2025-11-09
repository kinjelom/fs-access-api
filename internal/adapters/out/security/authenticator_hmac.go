package security

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"io"
	"net/http"
	"strings"
	"time"
)

type HMACAuthenticator struct {
	window time.Duration
	// accessSecrets maps public key-id -> secret bytes
	accessSecrets map[string][]byte
}

// Enforce compile-time conformance to the interface
var _ ports.Authenticator = (*HMACAuthenticator)(nil)

// Headers as constants for consistency
const (
	hmacScheme        = "HMAC"
	hmacHdrTimestamp  = "X-Timestamp"
	hmacHdrBodySHA256 = "X-Content-Sha256"
)

// Context key + helper if you want to pass identity down the stack.
type ctxKey string

func NewHMACAuthenticator(authCfg config.AuthenticatorConfig) (*HMACAuthenticator, error) {
	win := time.Duration(authCfg.WindowSeconds) * time.Second
	if win <= 0 {
		win = 5 * time.Minute
	}

	// decode hex secrets
	secrets := make(map[string][]byte, len(authCfg.AccessKeys))
	for keyID, hexSecret := range authCfg.AccessKeys {
		hexSecret = strings.TrimSpace(hexSecret)
		if hexSecret == "" {
			return nil, errors.New("empty secret for key " + keyID)
		}
		raw, err := hex.DecodeString(hexSecret)
		if err != nil {
			return nil, errors.New("invalid hex secret for key " + keyID + ": " + err.Error())
		}
		secrets[keyID] = raw
	}

	return &HMACAuthenticator{
		window:        win,
		accessSecrets: secrets,
	}, nil
}

func (s *HMACAuthenticator) Supports(r *http.Request) bool {
	authz := r.Header.Get(hdrAuthz)
	return strings.HasPrefix(authz, hmacScheme+" ")
}

// Verify does pure auth logic; no writes to ResponseWriter.
func (s *HMACAuthenticator) Verify(r *http.Request) error {
	apiKey := r.Header.Get(hdrAPIKey)
	authz := r.Header.Get(hdrAuthz)
	tsStr := r.Header.Get(hmacHdrTimestamp)
	bodySHA := r.Header.Get(hmacHdrBodySHA256)

	if apiKey == "" || authz == "" || tsStr == "" || bodySHA == "" {
		return fmt.Errorf("missing auth headers")
	}
	secret, ok := s.accessSecrets[apiKey]
	if !ok {
		return fmt.Errorf("unknown api key")
	}
	if !strings.HasPrefix(authz, "HMAC ") {
		return fmt.Errorf("invalid auth scheme")
	}
	sigHex := strings.TrimPrefix(authz, "HMAC ")

	// Timestamp window (replay)
	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		return fmt.Errorf("bad timestamp")
	}
	now := time.Now().UTC()
	if d := now.Sub(ts); d > s.window || d < -s.window {
		return fmt.Errorf("timestamp outside allowed window")
	}

	// Compute/verify body hash; restore body afterwards
	localHash, err := bodyHashAndRestore(r)
	if err != nil {
		return fmt.Errorf("body read error: %w", err)
	}
	if !strings.EqualFold(bodySHA, localHash) {
		return fmt.Errorf("body hash mismatch")
	}

	// Canonical path: prefer EscapedPath to preserve encoding, avoid Clean()
	pathWithQuery := r.URL.EscapedPath()
	if raw := r.URL.RawQuery; raw != "" {
		pathWithQuery = pathWithQuery + "?" + raw
	}

	canonical := strings.Join([]string{
		r.Method,
		pathWithQuery,
		tsStr,
		localHash,
	}, "\n")

	// expected signature (raw bytes)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	expected := mac.Sum(nil)

	provided, err := hex.DecodeString(sigHex)
	if err != nil {
		return fmt.Errorf("bad signature encoding")
	}
	if !hmac.Equal(provided, expected) {
		return fmt.Errorf("bad signature")
	}

	return nil
}

func (s *HMACAuthenticator) WithAuthChi(next http.Handler) http.Handler {
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

func bodyHashAndRestore(r *http.Request) (string, error) {
	var body []byte
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return "", err
		}
		body = b
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(b))
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
