package rest

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"fs-access-api/internal/adapters/in/rest/openapi" // generated
	"fs-access-api/internal/app/ports"
	"net/http"
)

func (s *DefaultRestServer) GenerateSecret(w http.ResponseWriter, _ *http.Request, params openapi.GenerateSecretParams) {
	size, secret, err := s.apis.GenerateSecret(params.Size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.GenerateSecretResponseBody{
		Hex:       hex.EncodeToString(secret),
		SizeBytes: size,
	})
	return
}

func (s *DefaultRestServer) ComputeHash(w http.ResponseWriter, r *http.Request) {
	if !isJSON(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var in openapi.ComputeHashRequestBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if in.Plaintext == nil {
		writeError(w, http.StatusBadRequest, "empty plaintext")
		return
	}

	alg, err := ports.ParseHashAlgo(string(in.Algorithm))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid algorithm: '%s'", in.Algorithm))
	}

	hash, err := s.apis.ComputeHash(*in.Plaintext, alg, in.Rounds, in.SaltLen)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, openapi.ComputeHashResponseBody{
		Algorithm: in.Algorithm,
		Hash:      hash,
	})
	return
}

func (s *DefaultRestServer) VerifyHash(w http.ResponseWriter, r *http.Request) {
	if !isJSON(r) {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var in openapi.VerifyHashRequestBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if in.Plaintext == nil {
		writeError(w, http.StatusBadRequest, "empty plaintext password")
		return
	}

	verified, algorithm, err := s.apis.VerifyHash(in.Hash, *in.Plaintext)

	response := openapi.VerifyHashResponseBody{
		Verified:          verified,
		DetectedAlgorithm: string(algorithm),
	}

	if err != nil {
		errMsg := err.Error()
		response.Error = &errMsg
	}

	writeJSON(w, http.StatusOK, response)
	return
}
