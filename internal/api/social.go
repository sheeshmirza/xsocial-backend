package api

import (
	"fmt"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/common"
)

func (s *Server) listSocialAccounts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	handles, err := s.handleRepo.ListByUserID(r.Context(), userOID, bson.M{}, 0, 100, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list social accounts")
		return
	}
	if handles == nil {
		handles = []models.SocialHandle{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data":  handles,
		"count": len(handles),
	})
}

func (s *Server) connectSocialPlatform(w http.ResponseWriter, r *http.Request) {
	platform := strings.ToLower(r.PathValue("platform"))
	if platform == "" {
		writeError(w, http.StatusBadRequest, "platform is required")
		return
	}

	uid := currentUserID(r)
	if uid == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	connectURL, err := s.socialService.GetConnectURL(uid, platform)
	if err != nil {
		writeApiError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"platform": platform,
		"authUrl":  connectURL,
		"url":      connectURL,
	})
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	platform := strings.ToLower(r.PathValue("platform"))
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "code and state query parameters are required")
		return
	}

	userID, err := common.ValidateOAuthState(state, s.cfg.JWTAccessSecret)
	if err != nil || userID == "" {
		writeError(w, http.StatusBadRequest, "invalid or expired oauth state parameter")
		return
	}

	handle, err := s.socialService.ConnectFromCallback(r.Context(), userID, platform, code, state)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("OAuth connection failed: %v", err))
		return
	}

	frontendURL := s.cfg.ClientURL
	if frontendURL != "" {
		http.Redirect(w, r, fmt.Sprintf("%s/dashboard/accounts?connected=%s", strings.TrimRight(frontendURL, "/"), platform), http.StatusTemporaryRedirect)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": fmt.Sprintf("%s account connected successfully", platform),
		"data":    handle,
	})
}

func (s *Server) refreshSocialToken(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		SocialID string `json:"socialId"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	socialOID, err := bson.ObjectIDFromHex(req.SocialID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid social account ID")
		return
	}

	refreshed, err := s.socialService.RefreshPlatformToken(r.Context(), userOID, socialOID)
	if err != nil {
		writeApiError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    refreshed,
	})
}

func (s *Server) disconnectSocialAccount(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	socialOID, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid social account ID")
		return
	}

	if err := s.socialService.DisconnectAccount(r.Context(), userOID, socialOID); err != nil {
		writeApiError(w, err)
		return
	}

	s.auditLogService.Log(r.Context(), &userOID, nil, currentUserID(r), "social.disconnect", "social_account", socialOID.Hex(), clientIP(r), r.UserAgent())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "social account disconnected successfully",
	})
}
