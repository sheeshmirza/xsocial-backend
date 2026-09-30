package api

import (
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
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

	s.auditLogService.Log(r.Context(), &userOID, nil, currentUserID(r), "api_key.create", "api_key", resp.ApiKey.ID, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
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
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	idStr := r.PathValue("id")
	keyOID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid api key ID")
		return
	}

	if err := s.apiKeyService.RevokeApiKey(r.Context(), keyOID, userOID); err != nil {
		writeApiError(w, err)
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, nil, currentUserID(r), "api_key.delete", "api_key", idStr, clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "API key revoked successfully",
	})
}
