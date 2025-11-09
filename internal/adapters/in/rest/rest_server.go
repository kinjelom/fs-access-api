package rest

import (
	"encoding/json"
	"fs-access-api/internal/app/config"
	"fs-access-api/internal/app/ports"
	"net/http"
	"strings"
	"time"

	"fs-access-api/internal/adapters/in/rest/openapi" // generated
)

type DefaultRestServer struct {
	apis          ports.ApiServer
	restCfg       config.HttpServerConfig
	authenticator ports.Authenticator
	actionMetrics ports.ActionMetrics
	startTime     time.Time
}

// Enforce compile-time conformance to a generated interface
var _ openapi.ServerInterface = (*DefaultRestServer)(nil)

func NewRestServer(cfg config.HttpServerConfig, apiServer ports.ApiServer, authenticator ports.Authenticator, metrics ports.ActionMetrics) (*DefaultRestServer, error) {
	return &DefaultRestServer{
		restCfg:       cfg,
		apis:          apiServer,
		authenticator: authenticator,
		actionMetrics: metrics,
		startTime:     time.Now().UTC(),
	}, nil
}

func (s *DefaultRestServer) Health(w http.ResponseWriter, _ *http.Request) {
	err := s.apis.HealthCheck()
	if err == nil {
		writeJSON(w, http.StatusOK, openapi.HealthStatusResponseBody{
			Banner:    s.restCfg.Banner,
			Reason:    nil,
			StartedAt: s.startTime,
			Healthy:   true,
			UptimeSec: int64(time.Since(s.startTime).Seconds()),
		})
		return
	} else {
		writeJSON(w, http.StatusServiceUnavailable, openapi.HealthStatusResponseBody{
			Banner:    s.restCfg.Banner,
			Reason:    ptr(err.Error()),
			StartedAt: s.startTime,
			Healthy:   false,
			UptimeSec: int64(time.Since(s.startTime).Seconds()),
		})
		return
	}
}

// "Authz" endpoints: server_authz.go
// "Crypto" endpoints: server_crypto.go
// "Groups" endpoints: server_groups.go
// "Users" endpoints: server_users.go

// helpers:

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func isJSON(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	// accept "application/json" with optional charset
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(ct)), "application/json")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, openapi.Error{
		Code:    http.StatusText(status),
		Message: msg,
	})
}
func writeAuthError(w http.ResponseWriter, err error) {
	writeError(w, http.StatusUnauthorized, err.Error())
}

func ptr[T any](v T) *T { return &v }
