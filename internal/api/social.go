package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/utils"
)

func (s *Server) listSocialAccounts(w http.ResponseWriter, r *http.Request) {
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	platformFilter := r.URL.Query().Get("platform")
	handles, total, err := s.socialService.ListAccounts(r.Context(), userOID, platformFilter, 0, 100, bson.D{{Key: "updatedAt", Value: -1}})
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
		"total": total,
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

	var req struct {
		Code        string `json:"code"`
		State       string `json:"state"`
		RedirectURI string `json:"redirectUri"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		_ = decode(w, r, 0, &req)
	}

	if req.RedirectURI != "" {
		if err := utils.ValidateURL(req.RedirectURI); err != nil {
			writeError(w, http.StatusBadRequest, "invalid redirectUri: "+err.Error())
			return
		}
	}

	if req.Code != "" {
		state := req.State
		if state == "" {
			state = common.GenerateOAuthState(uid, s.cfg.JWTAccessSecret)
		}
		var customURIs []string
		if req.RedirectURI != "" {
			customURIs = append(customURIs, req.RedirectURI)
		}
		handle, err := s.socialService.ConnectFromCallback(r.Context(), uid, platform, req.Code, state, customURIs...)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("OAuth connection failed: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": fmt.Sprintf("%s account connected successfully", platform),
			"data":    handle,
		})
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
		"data": map[string]any{
			"platform": platform,
			"authUrl":  connectURL,
			"url":      connectURL,
		},
	})
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	platform := strings.ToLower(r.PathValue("platform"))
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")
	errDesc := r.URL.Query().Get("error_description")

	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json"
	frontendURL := strings.TrimRight(s.cfg.ClientURL, "/")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	redirectError := func(errMsg string) {
		if isJSON {
			writeError(w, http.StatusBadRequest, errMsg)
		} else {
			http.Redirect(w, r, fmt.Sprintf("%s/accounts?connected=%s&status=error&message=%s", frontendURL, platform, url.QueryEscape(errMsg)), http.StatusTemporaryRedirect)
		}
	}

	if errParam != "" {
		msg := errDesc
		if msg == "" {
			msg = errParam
		}
		redirectError(msg)
		return
	}

	if code == "" || state == "" {
		redirectError("code and state query parameters are required")
		return
	}

	userID, err := common.ValidateOAuthState(state, s.cfg.JWTAccessSecret)
	if err != nil || userID == "" {
		redirectError("invalid or expired oauth state parameter")
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	currentCallbackURI := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL.Path)

	handle, err := s.socialService.ConnectFromCallback(r.Context(), userID, platform, code, state, currentCallbackURI)
	if err != nil {
		redirectError(fmt.Sprintf("OAuth connection failed: %v", err))
		return
	}

	if isJSON {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": fmt.Sprintf("%s account connected successfully", platform),
			"data":    handle,
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("%s/accounts?connected=%s&status=success", frontendURL, platform), http.StatusTemporaryRedirect)
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
	userOID, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	socialOID, ok := s.parsePathOID(w, r, "id", "social account")
	if !ok {
		return
	}

	if err := s.socialService.DisconnectAccount(r.Context(), userOID, socialOID); err != nil {
		writeApiError(w, err)
		return
	}

	s.logAudit(r, &userOID, nil, "social.disconnect", "social_account", socialOID.Hex())
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "social account disconnected successfully",
	})
}
