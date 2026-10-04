package api

import (
	"net/http"
	"strings"

	"xsocial-backend/internal/models"
)

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	var req models.CreateApiKeyRequest
	if !decode(w, r, 0, &req) {
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "Default API Key"
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusUnprocessableEntity, "key name must be at most 100 characters")
		return
	}

	resp, err := s.apiKeyService.CreateApiKey(r.Context(), userOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, nil, "api_key.create", "api_key", resp.ApiKey.ID)
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	keys, err := s.apiKeyService.ListApiKeys(r.Context(), userOID)
	if err != nil {
		writeApiError(w, err)
		return
	}
	if keys == nil {
		keys = []models.ApiKeyView{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  keys,
		"count": len(keys),
	})
}

func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	keyOID, ok := s.parsePathOID(w, r, "id", "api key")
	if !ok {
		return
	}

	if err := s.apiKeyService.RevokeApiKey(r.Context(), keyOID, userOID); err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, nil, "api_key.delete", "api_key", keyOID.Hex())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "API key revoked successfully",
	})
}
