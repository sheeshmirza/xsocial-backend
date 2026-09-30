package api

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) createWebhook(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.CreateWebhookRequest
	if !decode(w, r, 0, &req) {
		return
	}

	if req.BrandID == "" {
		if bOID, ok := currentBrandOID(r); ok {
			req.BrandID = bOID.Hex()
		}
	}

	wh, err := s.webhookService.CreateWebhook(r.Context(), userOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, wh.BrandID, currentUserID(r), "webhook.create", "webhook", wh.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, wh)
}

func (s *Server) listWebhooks(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var brandOIDPtr *bson.ObjectID
	if bOID, ok := currentBrandOID(r); ok {
		brandOIDPtr = &bOID
	}

	webhooks, err := s.webhookService.ListWebhooks(r.Context(), userOID, brandOIDPtr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list webhooks: "+err.Error())
		return
	}
	if webhooks == nil {
		webhooks = []models.Webhook{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  webhooks,
		"count": len(webhooks),
	})
}

func (s *Server) deleteWebhook(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	whOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	if err := s.webhookService.DeleteWebhook(r.Context(), whOID, userOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, nil, currentUserID(r), "webhook.delete", "webhook", whOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "webhook deleted successfully",
	})
}
