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

func (s *Server) createAdCampaign(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	var req models.CreateAdCampaignRequest
	if !decode(w, r, 0, &req) {
		return
	}

	camp, err := s.adService.CreateCampaign(r.Context(), brandOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	userOID, _ := currentUserOID(r)
	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "ads.create", "campaign", camp.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, camp)
}

func (s *Server) updateAdCampaign(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	campOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign ID")
		return
	}

	var req models.UpdateAdCampaignRequest
	if !decode(w, r, 0, &req) {
		return
	}

	updated, err := s.adService.UpdateCampaign(r.Context(), campOID, brandOID, req)
	if err != nil {
		writeApiError(w, err)
		return
	}

	userOID, _ := currentUserOID(r)
	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "ads.update", "campaign", updated.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) deleteAdCampaign(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	campOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid campaign ID")
		return
	}

	if err := s.adService.DeleteCampaign(r.Context(), campOID, brandOID); err != nil {
		writeApiError(w, err)
		return
	}

	userOID, _ := currentUserOID(r)
	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "ads.delete", "campaign", campOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

