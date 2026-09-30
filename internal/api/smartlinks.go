package api

import (
	"net/http"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func (s *Server) createSmartLink(w http.ResponseWriter, r *http.Request) {
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

	var req models.CreateSmartLinkRequest
	if !decode(w, r, 0, &req) {
		return
	}

	page, err := s.smartLinkService.CreatePage(r.Context(), brandOID, userOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, &brandOID, currentUserID(r), "smartlink.create", "smartlink_page", page.ID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusCreated, page)
}

func (s *Server) listSmartLinks(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	pages, err := s.smartLinkService.ListPagesByBrand(r.Context(), brandOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list smartlinks: "+err.Error())
		return
	}
	if pages == nil {
		pages = []models.SmartLinkPage{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  pages,
		"count": len(pages),
	})
}

func (s *Server) updateSmartLink(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	pageOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid smartlink ID")
		return
	}

	var req models.UpdateSmartLinkRequest
	if !decode(w, r, 0, &req) {
		return
	}

	page, err := s.smartLinkService.UpdatePage(r.Context(), pageOID, brandOID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func (s *Server) deleteSmartLink(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	pageOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid smartlink ID")
		return
	}

	if err := s.smartLinkService.DeletePage(r.Context(), pageOID, brandOID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "smartlink deleted successfully",
	})
}

func (s *Server) getSmartLinkAnalytics(w http.ResponseWriter, r *http.Request) {
	brandOID, err := s.resolveBrandForRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "brandId is required")
		return
	}

	pageOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid smartlink ID")
		return
	}

	analytics, err := s.smartLinkService.GetAnalytics(r.Context(), pageOID, brandOID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get analytics: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, analytics)
}

func (s *Server) publicBioPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	page, err := s.smartLinkService.GetPageBySlug(r.Context(), slug, true)
	if err != nil || page == nil {
		writeError(w, http.StatusNotFound, "bio page not found")
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func (s *Server) publicTrackClick(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	buttonID := r.PathValue("buttonId")

	page, err := s.smartLinkService.GetPageBySlug(r.Context(), slug, false)
	if err != nil || page == nil {
		writeError(w, http.StatusNotFound, "bio page not found")
		return
	}

	_ = s.smartLinkService.TrackButtonClick(r.Context(), slug, buttonID)

	var targetURL string
	for _, b := range page.Buttons {
		if b.ID == buttonID {
			targetURL = b.URL
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"targetUrl": targetURL,
	})
}
