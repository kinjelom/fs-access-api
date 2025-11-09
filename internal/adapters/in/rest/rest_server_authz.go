package rest

import (
	"errors"
	"fmt"
	"fs-access-api/internal/adapters/in/rest/openapi" // generated
	"fs-access-api/internal/adapters/out/metrics"
	"fs-access-api/internal/app/ports"
	"net/http"
)

func (s *DefaultRestServer) AuthzLookupUser(w http.ResponseWriter, r *http.Request, username openapi.UsernameParam) {
	aa := metrics.NewAuthzAction("lookup", username)
	if err := s.authenticator.Verify(r); err != nil {
		s.actionMetrics.OnActionDone(aa.Done(ports.MAResultUnauthorizedApiClient))
		writeAuthError(w, err) // 401
		return
	}

	uai, rootPath, err := s.apis.AuthzLookupUser(username)
	s.actionMetrics.OnActionDone(aa.DoneFromError(err))

	if err == nil {
		if uai == nil {
			writeError(w, http.StatusInternalServerError, "unexpected empty user info")
			return
		}
		w.Header().Set("X-FS-UID", fmt.Sprintf("%d", uai.UID))
		w.Header().Set("X-FS-GID", fmt.Sprintf("%d", uai.GID))
		w.Header().Set("X-FS-Dir", uai.AbsoluteHomeDir(rootPath))
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch {
	case errors.Is(err, ports.ErrNotFound):
		writeError(w, http.StatusNotFound, "user not found")
		return
	case errors.Is(err, ports.ErrLockedUser):
		writeError(w, http.StatusNotFound, "user not found")
		return
	case errors.Is(err, ports.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

func (s *DefaultRestServer) AuthzAuthUser(w http.ResponseWriter, r *http.Request, username openapi.UsernameParam) {
	aa := metrics.NewAuthzAction("auth", username)

	if err := s.authenticator.Verify(r); err != nil {
		s.actionMetrics.OnActionDone(aa.Done(ports.MAResultUnauthorizedApiClient))
		writeAuthError(w, err) // 401
		return
	}

	if err := r.ParseForm(); err != nil {
		s.actionMetrics.OnActionDone(aa.Done(ports.MAResultFailure))
		writeError(w, http.StatusBadRequest, "invalid form body")
		return
	}
	password := r.PostFormValue("password")
	if password == "" {
		s.actionMetrics.OnActionDone(aa.Done(ports.MAResultForbiddenUser))
		writeError(w, http.StatusForbidden, "authentication failed")
		return
	}

	err := s.apis.AuthzAuthUser(username, password)
	s.actionMetrics.OnActionDone(aa.DoneFromError(err))

	if err == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch {
	case errors.Is(err, ports.ErrInvalidCredentials),
		errors.Is(err, ports.ErrInvalidInput),
		errors.Is(err, ports.ErrNotFound):
		writeError(w, http.StatusForbidden, "authentication failed")
		return
	case errors.Is(err, ports.ErrLockedUser):
		writeError(w, http.StatusLocked, "user locked")
		return
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
}
