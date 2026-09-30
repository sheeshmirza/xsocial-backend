package api

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	_, limitNum := page(r)

	var brandOIDPtr *bson.ObjectID
	if bOID, ok := currentBrandOID(r); ok {
		brandOIDPtr = &bOID
	}

	logs, err := s.auditLogService.ListLogs(r.Context(), &userOID, brandOIDPtr, int64(limitNum))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs: "+err.Error())
		return
	}
	if logs == nil {
		logs = []models.AuditLog{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  logs,
		"count": len(logs),
	})
}
