package api

import (
	"net/http"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

var transparentPixel1x1 = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x21,
	0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
	0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

func (s *Server) getWebAnalytics(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	overview, err := s.analyticsService.GetWebOverview(r.Context(), brandOID, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get web analytics: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (s *Server) getSocialAnalytics(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}
	platform := r.URL.Query().Get("platform")
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	metrics, err := s.analyticsService.GetSocialOverview(r.Context(), brandOID, platform, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get social analytics: "+err.Error())
		return
	}
	if metrics == nil {
		metrics = []models.SocialMetricRollup{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data":  metrics,
		"count": len(metrics),
	})
}

func (s *Server) publicTrackerPixel(w http.ResponseWriter, r *http.Request) {
	brandIDStr := r.URL.Query().Get("brandId")
	if brandOID, err := bson.ObjectIDFromHex(brandIDStr); err == nil {
		url := r.URL.Query().Get("url")
		path := r.URL.Query().Get("path")
		ref := r.URL.Query().Get("ref")
		sessionID := r.URL.Query().Get("sid")
		_ = s.analyticsService.RecordWebHit(r.Context(), brandOID, sessionID, url, path, ref, "", "desktop", "browser")
	}
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(transparentPixel1x1)
}

func (s *Server) publicTrackerEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BrandID   string `json:"brandId"`
		SessionID string `json:"sessionId"`
		URL       string `json:"url"`
		Path      string `json:"path"`
		Referrer  string `json:"referrer"`
		Country   string `json:"country"`
		Device    string `json:"device"`
		Browser   string `json:"browser"`
	}
	if !decode(w, r, 0, &req) {
		return
	}
	brandOID, err := bson.ObjectIDFromHex(req.BrandID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid brand ID")
		return
	}
	if err := s.analyticsService.RecordWebHit(r.Context(), brandOID, req.SessionID, req.URL, req.Path, req.Referrer, req.Country, req.Device, req.Browser); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record analytics hit")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
	})
}

func (s *Server) recordSocialRollup(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}
	var rollup models.SocialMetricRollup
	if !decode(w, r, 0, &rollup) {
		return
	}
	rollup.BrandID = brandOID
	if err := s.analyticsService.RecordSocialMetricRollup(r.Context(), &rollup); err != nil {
		writeApiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
