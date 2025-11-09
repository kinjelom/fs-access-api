package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"fs-access-api/internal/adapters/in/rest/openapi" // generated
	"fs-access-api/internal/app/ports"
	"net/http"
	"net/url"
	"strings"
)

func (s *DefaultRestServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	items, err := s.apis.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list users: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
	return
}

func (s *DefaultRestServer) EnsureUser(w http.ResponseWriter, r *http.Request, name openapi.UsernameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	if !isJSON(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var in openapi.EnsureUserRequestBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if in.Password == nil || len(strings.TrimSpace(*in.Password)) == 0 {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	home := name
	if in.Home != nil {
		home = *in.Home
	}
	disabled := false
	if in.Disabled != nil {
		disabled = *in.Disabled
	}

	ru := ports.UserInfo{
		Username:       name,
		UID:            0,
		Groupname:      in.Groupname,
		Password:       *in.Password,
		PasswordIsHash: in.PasswordIsHash != nil && *in.PasswordIsHash,
		Description:    in.Description,
		Home:           home,
		Expiration:     in.Expiration,
		Disabled:       disabled,
	}

	_, created, err := s.apis.EnsureUser(ru)
	if err != nil {
		if errors.Is(err, ports.ErrConflict) {
			writeJSON(w, http.StatusConflict, openapi.Conflict{
				Code:    "USER_CONFLICT",
				Message: "User exists with different attributes",
			})
			return
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot ensure user: %v", err))
			return
		}
	}

	w.Header().Set("Location", fmt.Sprintf("/api/users/%s", url.PathEscape(name)))
	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}

}

func (s *DefaultRestServer) GetUser(w http.ResponseWriter, r *http.Request, name openapi.UsernameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	u, err := s.apis.GetUser(name)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, u)
	return
}

func (s *DefaultRestServer) SetUserDescription(w http.ResponseWriter, r *http.Request, name openapi.UsernameParam) {
	handleUserAttributesUpdate[openapi.SetDescriptionRequestBody](s, w, r, name, func(u ports.UserInfo, in openapi.SetDescriptionRequestBody) (ports.UserInfo, error) {
		u.Description = in.Description
		return u, nil
	})
}

func (s *DefaultRestServer) SetUserPassword(w http.ResponseWriter, r *http.Request, name openapi.UsernameParam) {
	handleUserAttributesUpdate[openapi.SetUserPasswordRequestBody](s, w, r, name, func(u ports.UserInfo, in openapi.SetUserPasswordRequestBody) (ports.UserInfo, error) {
		if in.Password == nil || len(strings.TrimSpace(*in.Password)) == 0 {
			return u, fmt.Errorf("password is required")
		}
		u.Password = *in.Password
		u.PasswordIsHash = in.PasswordIsHash != nil && *in.PasswordIsHash
		return u, nil
	})
}

func (s *DefaultRestServer) SetUserExpiration(w http.ResponseWriter, r *http.Request, name openapi.UsernameParam) {
	handleUserAttributesUpdate[openapi.SetUserExpirationRequestBody](s, w, r, name, func(u ports.UserInfo, in openapi.SetUserExpirationRequestBody) (ports.UserInfo, error) {
		u.Expiration = in.Expiration
		return u, nil
	})
}

func (s *DefaultRestServer) SetUserDisabled(w http.ResponseWriter, r *http.Request, name openapi.UsernameParam) {
	handleUserAttributesUpdate[openapi.SetUserDisabledRequestBody](s, w, r, name, func(u ports.UserInfo, in openapi.SetUserDisabledRequestBody) (ports.UserInfo, error) {
		u.Disabled = in.Disabled
		return u, nil
	})
}

func (s *DefaultRestServer) DeleteUser(w http.ResponseWriter, r *http.Request, name openapi.UsernameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}

	// Fetch the existing user
	err := s.apis.DeleteUser(name)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
	return
}

func (s *DefaultRestServer) ListUserDirs(w http.ResponseWriter, r *http.Request, username openapi.UsernameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	dirs, err := s.apis.ListUserDirs(username)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dirs)
}

func (s *DefaultRestServer) DeleteUserDir(w http.ResponseWriter, r *http.Request, username openapi.UsernameParam, dirname openapi.DirnameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	err := s.apis.DeleteUserDir(username, dirname)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user or directory not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return
}

func (s *DefaultRestServer) EnsureUserDir(w http.ResponseWriter, r *http.Request, username openapi.UsernameParam, dirname openapi.DirnameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	created, err := s.apis.EnsureUserDir(username, dirname)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/api/users/%s/directories/%s", url.PathEscape(username), url.PathEscape(dirname)))
	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func handleUserAttributesUpdate[T any](s *DefaultRestServer, w http.ResponseWriter, r *http.Request, name string, mutate func(u ports.UserInfo, in T) (ports.UserInfo, error)) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	if !isJSON(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var in T
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	err := s.apis.UpdateUser(name, func(u ports.UserInfo) (ports.UserInfo, error) {
		return mutate(u, in)
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return
}
