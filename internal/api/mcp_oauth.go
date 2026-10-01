package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	mcpOAuthScope           = "xsocial"
	mcpOAuthDefaultResource = "https://xs-be-sa1e.onrender.com"
	mcpOAuthDefaultIssuer   = "https://xs-be-sa1e.onrender.com"
)

var usedMCPAuthCodes sync.Map
var usedMCPRefreshTokens sync.Map

func (s *Server) mcpOAuthResource(r *http.Request) string {
	if s.cfg.MCPOAuthResource != "" {
		return s.cfg.MCPOAuthResource
	}
	if s.cfg.PublicBaseURL != "" {
		return strings.TrimRight(s.cfg.PublicBaseURL, "/")
	}
	return mcpOAuthDefaultResource
}

func (s *Server) mcpOAuthIssuer(r *http.Request) string {
	if s.cfg.MCPOAuthIssuer != "" {
		return s.cfg.MCPOAuthIssuer
	}
	if s.cfg.PublicBaseURL != "" {
		return strings.TrimRight(s.cfg.PublicBaseURL, "/")
	}
	return mcpOAuthDefaultIssuer
}

func validMCPClient(clientID string) bool {
	return clientID == "https://chatgpt.com/oauth/client.json" ||
		(strings.HasPrefix(clientID, "https://chatgpt.com/oauth/") && strings.HasSuffix(clientID, "/client.json"))
}

func validMCPRedirect(redirectURI string) bool {
	if redirectURI == "https://chatgpt.com/connector_platform_oauth_redirect" {
		return true
	}
	return strings.HasPrefix(redirectURI, "https://chatgpt.com/connector/oauth/")
}

func randomMCPString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Server) mcpProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	resource := s.mcpOAuthResource(r)
	issuer := s.mcpOAuthIssuer(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"resource":              resource,
		"authorization_servers": []string{issuer},
		"scopes_supported":      []string{mcpOAuthScope},
	})
}

func (s *Server) mcpAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	issuer := s.mcpOAuthIssuer(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                      issuer,
		"authorization_response_iss_parameter_supported": true,
		"authorization_endpoint":                         s.mcpOAuthAuthorizationEndpoint(),
		"token_endpoint":                                 issuer + "/oauth/token",
		"client_id_metadata_document_supported":          true,
		"token_endpoint_auth_methods_supported":          []string{"none"},
		"code_challenge_methods_supported":               []string{"S256"},
		"scopes_supported":                               []string{mcpOAuthScope},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"response_types_supported":                       []string{"code"},
	})
}

type mcpOAuthTransaction struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Resource            string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Issuer              string
}

func (s *Server) mcpOAuthAuthorizationEndpoint() string {
	if s.cfg.ClientURL != "" {
		return strings.TrimRight(s.cfg.ClientURL, "/") + "/oauth/authorize/"
	}
	return s.mcpOAuthIssuer(nil) + "/oauth/authorize"
}

func (s *Server) signMCPTransaction(tx mcpOAuthTransaction, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"mcp_oauth_tx": true, "response_type": tx.ResponseType, "client_id": tx.ClientID, "redirect_uri": tx.RedirectURI,
		"resource": tx.Resource, "scope": tx.Scope, "state": tx.State, "code_challenge": tx.CodeChallenge,
		"code_challenge_method": tx.CodeChallengeMethod, "iss": tx.Issuer,
		"iat": now.Unix(), "exp": now.Add(ttl).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTAccessSecret))
}

func (s *Server) parseMCPTransaction(raw string) (mcpOAuthTransaction, bool) {
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTAccessSecret), nil
	})
	if err != nil || !token.Valid {
		return mcpOAuthTransaction{}, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["mcp_oauth_tx"] != true || claims["iss"] != s.mcpOAuthIssuer(nil) {
		return mcpOAuthTransaction{}, false
	}
	get := func(key string) string { value, _ := claims[key].(string); return value }
	tx := mcpOAuthTransaction{
		ResponseType: get("response_type"), ClientID: get("client_id"), RedirectURI: get("redirect_uri"), Resource: get("resource"),
		Scope: get("scope"), State: get("state"), CodeChallenge: get("code_challenge"), CodeChallengeMethod: get("code_challenge_method"),
		Issuer: get("iss"),
	}
	if tx.ResponseType != "code" || !validMCPClient(tx.ClientID) || !validMCPRedirect(tx.RedirectURI) ||
		tx.Resource != s.mcpOAuthResource(nil) || tx.CodeChallenge == "" || tx.CodeChallengeMethod != "S256" {
		return mcpOAuthTransaction{}, false
	}
	return tx, true
}

func (s *Server) authenticateMCPUserToken(r *http.Request) (bson.ObjectID, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return bson.NilObjectID, false
	}
	raw := strings.TrimSpace(auth[7:])
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTAccessSecret), nil
	})
	if err != nil || !token.Valid {
		return bson.NilObjectID, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return bson.NilObjectID, false
	}
	sub, _ := claims["sub"].(string)
	oid, err := bson.ObjectIDFromHex(sub)
	if err != nil {
		return bson.NilObjectID, false
	}
	return oid, true
}

func (s *Server) oauthRedirectResult(r *http.Request, redirectURI, state string, code, description string) string {
	u, _ := url.Parse(redirectURI)
	p := u.Query()
	if code != "" {
		p.Set("error", code)
		p.Set("error_description", description)
	} else {
		p.Set("error", "access_denied")
		p.Set("error_description", "user denied access")
	}
	if state != "" {
		p.Set("state", state)
	}
	p.Set("iss", s.mcpOAuthIssuer(r))
	u.RawQuery = p.Encode()
	return u.String()
}

func (s *Server) mcpAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		tx := mcpOAuthTransaction{
			ResponseType: q.Get("response_type"),
			ClientID: q.Get("client_id"),
			RedirectURI: q.Get("redirect_uri"),
			Resource: q.Get("resource"),
			Scope: strings.TrimSpace(q.Get("scope")),
			State: q.Get("state"),
			CodeChallenge: q.Get("code_challenge"),
			CodeChallengeMethod: q.Get("code_challenge_method"),
			Issuer: s.mcpOAuthIssuer(r),
		}
		if tx.Resource == "" { tx.Resource = s.mcpOAuthResource(r) }
		if tx.Scope == "" { tx.Scope = mcpOAuthScope }
		if tx.ResponseType != "code" || !validMCPClient(tx.ClientID) || !validMCPRedirect(tx.RedirectURI) ||
			tx.CodeChallengeMethod != "S256" || tx.CodeChallenge == "" || tx.Resource != s.mcpOAuthResource(r) {
			s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "invalid_request", "invalid OAuth authorization request")
			return
		}
		for _, part := range strings.Fields(tx.Scope) {
			if part != mcpOAuthScope {
				s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "invalid_scope", "unsupported scope")
				return
			}
		}
		rawTx, err := s.signMCPTransaction(tx, 10*time.Minute)
		if err != nil {
			s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "server_error", "could not start authorization")
			return
		}
		frontendURL := strings.TrimRight(s.cfg.ClientURL, "/")
		if frontendURL == "" {
			s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "server_error", "OAuth UI is not configured")
			return
		}
		u, _ := url.Parse(frontendURL + "/oauth/authorize/")
		p := u.Query()
		p.Set("transaction", rawTx)
		u.RawQuery = p.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	rawTx := strings.TrimSpace(r.FormValue("transaction"))
	tx, ok := s.parseMCPTransaction(rawTx)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	action := r.FormValue("action")
	if action == "deny" {
		writeJSON(w, http.StatusOK, map[string]any{
			"redirect_uri": s.oauthRedirectResult(r, tx.RedirectURI, tx.State, "access_denied", "user denied access"),
		})
		return
	}
	if action != "allow" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}

	userID, ok := s.authenticateMCPUserToken(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
		return
	}

	jti, err := randomMCPString(24)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"mcp_code": true, "sub": userID.Hex(), "client_id": tx.ClientID, "redirect_uri": tx.RedirectURI, "resource": tx.Resource,
		"code_challenge": tx.CodeChallenge, "code_challenge_method": "S256", "scope": tx.Scope, "jti": jti,
		"iat": now.Unix(), "exp": now.Add(2*time.Minute).Unix(), "iss": s.mcpOAuthIssuer(r),
	}
	code, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}
	u, _ := url.Parse(tx.RedirectURI)
	p := u.Query()
	p.Set("code", code)
	if tx.State != "" { p.Set("state", tx.State) }
	p.Set("iss", s.mcpOAuthIssuer(r))
	u.RawQuery = p.Encode()
	writeJSON(w, http.StatusOK, map[string]any{"redirect_uri": u.String()})
}

func (s *Server) mcpToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}

	if r.Form.Get("grant_type") != "authorization_code" {
		if r.Form.Get("grant_type") != "refresh_token" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
			return
		}
		s.mcpRefreshToken(w, r)
		return
	}

	code := r.Form.Get("code")
	redirectURI := r.Form.Get("redirect_uri")
	clientID := r.Form.Get("client_id")
	verifier := r.Form.Get("code_verifier")
	resource := r.Form.Get("resource")
	if code == "" || redirectURI == "" || clientID == "" || verifier == "" || resource != s.mcpOAuthResource(r) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	if !validMCPClient(clientID) || !validMCPRedirect(redirectURI) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}

	token, err := jwt.Parse(code, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTAccessSecret), nil
	})
	if err != nil || !token.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["mcp_code"] != true {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	if claims["iss"] != s.mcpOAuthIssuer(r) ||
		claims["client_id"] != clientID ||
		claims["redirect_uri"] != redirectURI ||
		claims["resource"] != resource ||
		claims["code_challenge_method"] != "S256" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	jti, _ := claims["jti"].(string)
	if jti == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	challenge, _ := claims["code_challenge"].(string)
	sum := sha256.Sum256([]byte(verifier))
	if !strings.EqualFold(base64.RawURLEncoding.EncodeToString(sum[:]), challenge) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	if s.redis != nil {
		ok, err := s.redis.SetNX(r.Context(), "mcp:oauth:code:"+jti, "1", 2*time.Minute)
		if err != nil || !ok {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
	} else {
		if _, loaded := usedMCPAuthCodes.LoadOrStore(jti, time.Now()); loaded {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
	}

	sub, _ := claims["sub"].(string)
	if _, err := bson.ObjectIDFromHex(sub); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	now := time.Now()
	accessClaims := jwt.MapClaims{
		"mcp":   true,
		"sub":   sub,
		"aud":   resource,
		"scope": scopeFromMCPClaims(claims),
		"iss":   s.mcpOAuthIssuer(r),
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}

	refreshJTI, err := randomMCPString(24)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}
	refreshClaims := jwt.MapClaims{
		"mcp_refresh": true,
		"sub":         sub,
		"aud":         resource,
		"scope":       scopeFromMCPClaims(claims),
		"jti":         refreshJTI,
		"iss":         s.mcpOAuthIssuer(r),
		"iat":         now.Unix(),
		"exp":         now.Add(30 * 24 * time.Hour).Unix(),
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}

	if s.redis != nil {
		ok, err := s.redis.SetNX(r.Context(), "mcp:oauth:refresh:"+refreshJTI, sub, 30*24*time.Hour)
		if err != nil || !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
			return
		}
	} else {
		usedMCPRefreshTokens.Delete(refreshJTI)
		usedMCPRefreshTokens.LoadOrStore(refreshJTI, sub)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"scope":         scopeFromMCPClaims(claims),
	})
}

func scopeFromMCPClaims(claims jwt.MapClaims) string {
	scope, _ := claims["scope"].(string)
	if scope == "" {
		return mcpOAuthScope
	}
	return scope
}

func (s *Server) mcpRefreshToken(w http.ResponseWriter, r *http.Request) {
	clientID := r.Form.Get("client_id")
	resource := r.Form.Get("resource")
	raw := strings.TrimSpace(r.Form.Get("refresh_token"))
	if raw == "" || !validMCPClient(clientID) || resource != s.mcpOAuthResource(r) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}

	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTAccessSecret), nil
	})
	if err != nil || !token.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["mcp_refresh"] != true ||
		claims["iss"] != s.mcpOAuthIssuer(r) ||
		claims["aud"] != s.mcpOAuthResource(r) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	refreshJTI, _ := claims["jti"].(string)
	sub, _ := claims["sub"].(string)
	if refreshJTI == "" || sub == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	if _, err := bson.ObjectIDFromHex(sub); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	if s.redis != nil {
		stored, err := s.redis.Get(r.Context(), "mcp:oauth:refresh:"+refreshJTI)
		if err != nil || stored != sub {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
		used, err := s.redis.SetNX(r.Context(), "mcp:oauth:refresh:"+refreshJTI+":used", "1", 30*24*time.Hour)
		if err != nil || !used {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
	} else {
		if _, loaded := usedMCPRefreshTokens.LoadOrStore(refreshJTI, sub); loaded {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
	}

	scope := scopeFromMCPClaims(claims)
	now := time.Now()
	accessClaims := jwt.MapClaims{
		"mcp": true, "sub": sub, "aud": resource, "scope": scope,
		"iss": s.mcpOAuthIssuer(r), "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}

	newRefreshJTI, err := randomMCPString(24)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}
	newRefreshClaims := jwt.MapClaims{
		"mcp_refresh": true, "sub": sub, "aud": resource, "scope": scope,
		"jti": newRefreshJTI, "iss": s.mcpOAuthIssuer(r), "iat": now.Unix(), "exp": now.Add(30 * 24 * time.Hour).Unix(),
	}
	newRefreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, newRefreshClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}
	if s.redis != nil {
		ok, err := s.redis.SetNX(r.Context(), "mcp:oauth:refresh:"+newRefreshJTI, sub, 30*24*time.Hour)
		if err != nil || !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
			return
		}
	} else {
		usedMCPRefreshTokens.LoadOrStore(newRefreshJTI, sub)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"refresh_token": newRefreshToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
		"scope":        scope,
	})
}

func (s *Server) mcpOAuthError(w http.ResponseWriter, r *http.Request, redirectURI, state, code, description string) {
	if validMCPRedirect(redirectURI) {
		u, _ := url.Parse(redirectURI)
		p := u.Query()
		p.Set("error", code)
		p.Set("error_description", description)
		if state != "" {
			p.Set("state", state)
		}
		p.Set("iss", s.mcpOAuthIssuer(r))
		u.RawQuery = p.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	http.Error(w, description, http.StatusBadRequest)
}

func (s *Server) authenticateMCPBearer(r *http.Request) (bson.ObjectID, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return bson.NilObjectID, false
	}

	raw := strings.TrimSpace(auth[7:])
	token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(s.cfg.JWTAccessSecret), nil
	})
	if err != nil || !token.Valid {
		return bson.NilObjectID, false
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["mcp"] != true ||
		claims["iss"] != s.mcpOAuthIssuer(r) ||
		claims["aud"] != s.mcpOAuthResource(r) {
		return bson.NilObjectID, false
	}

	scope, _ := claims["scope"].(string)
	if !strings.Contains(" "+scope+" ", " "+mcpOAuthScope+" ") {
		return bson.NilObjectID, false
	}

	sub, _ := claims["sub"].(string)
	oid, err := bson.ObjectIDFromHex(sub)
	if err != nil {
		return bson.NilObjectID, false
	}
	return oid, true
}

func (s *Server) mcpAuthChallenge(w http.ResponseWriter, r *http.Request, id any) {
	resourceMeta := s.mcpOAuthResource(r) + "/.well-known/oauth-protected-resource"
	challenge := fmt.Sprintf("Bearer resource_metadata=%q, scope=%q, error=%q, error_description=%q", resourceMeta, mcpOAuthScope, "invalid_token", "OAuth authentication required")
	w.Header().Set("WWW-Authenticate", challenge)
	writeJSON(w, http.StatusOK, mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"isError": true,
			"content": []map[string]string{{"type": "text", "text": "OAuth authentication required."}},
			"_meta": map[string]any{
				"mcp/www_authenticate": []string{challenge},
			},
		},
	})
}

func mcpOAuthJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
