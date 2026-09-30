package api

import (
	"net/http"

	"xsocial-backend/internal/models"
)

func (s *Server) listAdCampaigns(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	platform := r.URL.Query().Get("platform")
	campaigns, err := s.adService.ListCampaigns(r.Context(), brandOID, platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list ad campaigns: "+err.Error())
		return
	}
	if campaigns == nil {
		campaigns = []models.AdCampaign{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  campaigns,
		"count": len(campaigns),
	})
}

func (s *Server) getAdSummary(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	summary, err := s.adService.GetSummary(r.Context(), brandOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get ad summary: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) syncAdCampaigns(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	var req struct {
		Platform string `json:"platform"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		_ = decode(w, r, 0, &req)
	}

	if err := s.adService.SyncAdAccounts(r.Context(), brandOID, req.Platform); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sync ad campaigns: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "ad campaigns synced successfully",
	})
}
