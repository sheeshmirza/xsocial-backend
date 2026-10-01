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
	mcpOAuthScope = "xsocial"
	mcpOAuthDefaultResource = "https://xs-be-sa1e.onrender.com"
	mcpOAuthDefaultIssuer   = "https://xs-be-sa1e.onrender.com"
)

var usedMCPAuthCodes sync.Map

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
		strings.HasPrefix(clientID, "https://chatgpt.com/oauth/") && strings.HasSuffix(clientID, "/client.json")
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
		"resource":             resource,
		"authorization_servers": []string{issuer},
		"scopes_supported":     []string{mcpOAuthScope},
	})
}

func (s *Server) mcpAuthorizationServerMetadata(w http.ResponseWriter, r *http.Request) {
	issuer := s.mcpOAuthIssuer(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                  issuer,
		"authorization_response_iss_parameter_supported": true,
		"authorization_endpoint":                  issuer + "/oauth/authorize",
		"token_endpoint":                          issuer + "/oauth/token",
		"client_id_metadata_document_supported":   true,
		"token_endpoint_auth_methods_supported":   []string{"none"},
		"code_challenge_methods_supported":        []string{"S256"},
		"scopes_supported":                        []string{mcpOAuthScope},
	})
}

func (s *Server) mcpAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("response_type") != "code" ||
		!validMCPClient(q.Get("client_id")) ||
		!validMCPRedirect(q.Get("redirect_uri")) ||
		q.Get("code_challenge_method") != "S256" ||
		q.Get("code_challenge") == "" {
		s.mcpOAuthError(w, r, q.Get("redirect_uri"), q.Get("state"), "invalid_request", "invalid OAuth authorization request")
		return
	}

	resource := q.Get("resource")
	if resource == "" {
		resource = s.mcpOAuthResource(r)
	}
	if resource != s.mcpOAuthResource(r) {
		s.mcpOAuthError(w, r, q.Get("redirect_uri"), q.Get("state"), "invalid_target", "invalid resource")
		return
	}

	userID := strings.TrimSpace(s.cfg.MCPOAuthUserID)
	if _, err := bson.ObjectIDFromHex(userID); err != nil {
		s.mcpOAuthError(w, r, q.Get("redirect_uri"), q.Get("state"), "server_error", "MCP OAuth is not configured with an xSocial user")
		return
	}

	scope := mcpOAuthScope
	if requested := strings.TrimSpace(q.Get("scope")); requested != "" {
		for _, part := range strings.Fields(requested) {
			if part != mcpOAuthScope {
				s.mcpOAuthError(w, r, q.Get("redirect_uri"), q.Get("state"), "invalid_scope", "unsupported scope")
				return
			}
		}
	}

	jti, err := randomMCPString(24)
	if err != nil {
		s.mcpOAuthError(w, r, q.Get("redirect_uri"), q.Get("state"), "server_error", "could not create authorization code")
		return
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"mcp_code":      true,
		"sub":           userID,
		"client_id":     q.Get("client_id"),
		"redirect_uri":  q.Get("redirect_uri"),
		"resource":      resource,
		"code_challenge": q.Get("code_challenge"),
		"scope":         scope,
		"jti":           jti,
		"iat":           now.Unix(),
		"exp":           now.Add(2 * time.Minute).Unix(),
		"iss":           s.mcpOAuthIssuer(r),
	}
	code, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		s.mcpOAuthError(w, r, q.Get("redirect_uri"), q.Get("state"), "server_error", "could not sign authorization code")
		return
	}

	u, _ := url.Parse(q.Get("redirect_uri"))
	p := u.Query()
	p.Set("code", code)
	if state := q.Get("state"); state != "" {
		p.Set("state", state)
	}
	p.Set("iss", s.mcpOAuthIssuer(r))
	u.RawQuery = p.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (s *Server) mcpToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	if r.Form.Get("grant_type") != "authorization_code" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
		return
	}

	code := r.Form.Get("code")
	redirectURI := r.Form.Get("redirect_uri")
	clientID := r.Form.Get("client_id")
	verifier := r.Form.Get("code_verifier")
	resource := r.Form.Get("resource")
	if code == "" || redirectURI == "" || clientID == "" || verifier == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	if !validMCPClient(clientID) || !validMCPRedirect(redirectURI) || resource != s.mcpOAuthResource(r) {
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
	if claims["iss"] != s.mcpOAuthIssuer(r) || claims["client_id"] != clientID ||
		claims["redirect_uri"] != redirectURI || claims["resource"] != resource {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	jti, _ := claims["jti"].(string)
	if jti == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	if _, loaded := usedMCPAuthCodes.LoadOrStore(jti, time.Now()); loaded {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	challenge, _ := claims["code_challenge"].(string)
	sum := sha256.Sum256([]byte(verifier))
	if !strings.EqualFold(base64.RawURLEncoding.EncodeToString(sum[:]), challenge) {
		usedMCPAuthCodes.Delete(jti)
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	sub, _ := claims["sub"].(string)
	if _, err := bson.ObjectIDFromHex(sub); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}

	now := time.Now()
	accessClaims := jwt.MapClaims{
		"mcp":    true,
		"sub":   sub,
		"aud":   resource,
		"scope": mcpOAuthScope,
		"iss":   s.mcpOAuthIssuer(r),
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   3600,
		"scope":        mcpOAuthScope,
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
	if !ok || claims["mcp"] != true || claims["iss"] != s.mcpOAuthIssuer(r) || claims["aud"] != s.mcpOAuthResource(r) {
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

func (s *Server) mcpAuthChallenge(w http.ResponseWriter, r *http.Request) {
	resourceMeta := s.mcpOAuthResource(r) + "/.well-known/oauth-protected-resource"
	w.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer resource_metadata=%q, scope=%q", resourceMeta, mcpOAuthScope))
	writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized", "message": "OAuth authentication required"})
}

func mcpOAuthJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
