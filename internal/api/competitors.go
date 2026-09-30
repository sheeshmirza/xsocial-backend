package api

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) addCompetitor(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	var req models.AddCompetitorRequest
	if !decode(w, r, 0, &req) {
		return
	}

	comp, err := s.competitorServ.AddCompetitor(r.Context(), brandOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "competitor.add", "competitor", comp.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, comp)
}

func (s *Server) listCompetitors(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	comps, err := s.competitorServ.ListCompetitors(r.Context(), brandOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list competitors: "+err.Error())
		return
	}
	if comps == nil {
		comps = []models.Competitor{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  comps,
		"count": len(comps),
	})
}

func (s *Server) deleteCompetitor(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	compOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid competitor ID")
		return
	}

	if err := s.competitorServ.DeleteCompetitor(r.Context(), compOID, brandOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "competitor deleted successfully",
	})
}

func (s *Server) syncCompetitor(w http.ResponseWriter, r *http.Request) {
	compOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid competitor ID")
		return
	}

	if err := s.competitorServ.SyncCompetitorMetrics(r.Context(), compOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "competitor metrics synchronized",
	})
}
