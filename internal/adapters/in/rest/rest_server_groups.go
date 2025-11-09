package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"fs-access-api/internal/adapters/in/rest/openapi" // generated
	"fs-access-api/internal/app/ports"
	"net/http"
	"net/url"
)

func (s *DefaultRestServer) ListGroups(w http.ResponseWriter, r *http.Request) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	items, err := s.apis.ListGroups()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list groups: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
	return
}

func (s *DefaultRestServer) EnsureGroup(w http.ResponseWriter, r *http.Request, name openapi.GroupnameParam) {
	// Auth
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	// Content-Type
	if !isJSON(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	// Decode request (EnsureGroupRequest: gid + members)
	var in openapi.EnsureGroupRequestBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	home := name
	if in.Home != nil {
		home = *in.Home
	}

	// Map to the domain model (name pochodzi z path param)
	gReq := ports.GroupInfo{
		Groupname:   name,
		GID:         in.Gid,
		Description: in.Description,
		Home:        home,
	}

	_, created, err := s.apis.EnsureGroup(gReq)
	if err != nil {
		if errors.Is(err, ports.ErrConflict) {
			writeJSON(w, http.StatusConflict, openapi.Conflict{
				Code:    "GROUP_CONFLICT",
				Message: "Group exists with different attributes",
			})
			return
		} else {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot ensure group: %v", err))
			return
		}
	}

	w.Header().Set("Location", fmt.Sprintf("/api/groups/%s", url.PathEscape(name)))
	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *DefaultRestServer) GetGroup(w http.ResponseWriter, r *http.Request, name openapi.GroupnameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	g, err := s.apis.GetGroup(name)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "group not found")
			return
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, g)
	return
}

func (s *DefaultRestServer) SetGroupDescription(w http.ResponseWriter, r *http.Request, name openapi.GroupnameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	if !isJSON(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var in openapi.SetDescriptionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	err := s.apis.UpdateGroup(name, func(group ports.GroupInfo) (ports.GroupInfo, error) {
		group.Description = in.Description
		return group, nil
	})

	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			writeError(w, http.StatusNotFound, "group not found")
			return
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)

}

func (s *DefaultRestServer) DeleteGroup(w http.ResponseWriter, r *http.Request, name openapi.GroupnameParam) {
	if err := s.authenticator.Verify(r); err != nil {
		writeAuthError(w, err)
		return
	}
	err := s.apis.DeleteGroup(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
	return
}
