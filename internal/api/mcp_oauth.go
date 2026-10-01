package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
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
		"authorization_endpoint":                         issuer + "/oauth/authorize",
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
	Phase               string
	UserID              string
}

type mcpOAuthPageData struct {
	Title       string
	Transaction string
	Error       string
	Name        string
	Email       string
}

var mcpOAuthPageTemplate = template.Must(template.New("mcp-oauth").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<style>
body{font-family:Inter,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f6f7fb;margin:0;color:#17181c}
.wrap{max-width:440px;margin:8vh auto;padding:24px}.card{background:#fff;border:1px solid #e6e7eb;border-radius:18px;padding:30px;box-shadow:0 10px 35px rgba(0,0,0,.07)}
h1{font-size:24px;margin:0 0 8px}p{line-height:1.5;color:#565a63}label{display:block;font-size:14px;font-weight:600;margin:16px 0 7px}
input{width:100%;box-sizing:border-box;padding:12px 13px;border:1px solid #d9dce2;border-radius:10px;font-size:15px}
button{border:0;border-radius:10px;padding:12px 16px;font-weight:700;font-size:15px;cursor:pointer}
.primary{background:#111827;color:#fff;width:100%;margin-top:18px}.secondary{background:#eef0f3;color:#222;width:100%;margin-top:10px}
.error{background:#fff1f1;color:#9b1c1c;border:1px solid #ffd0d0;padding:11px 12px;border-radius:10px;margin:15px 0}.small{font-size:12px;color:#7b7f87}
.scope{margin:16px 0;padding:14px;background:#f5f6f8;border-radius:10px}
</style></head>
<body><div class="wrap"><div class="card">
<h1>{{.Title}}</h1>
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
{{if eq .Title "Connect xSocial"}}
<p>Sign in to the xSocial account you want to connect to ChatGPT.</p>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="transaction" value="{{.Transaction}}">
<input type="hidden" name="action" value="login">
<label>Email</label><input name="email" type="email" autocomplete="email" required>
<label>Password</label><input name="password" type="password" autocomplete="current-password" required>
<button class="primary" type="submit">Continue</button>
</form>
<p class="small">Your credentials are sent only to xSocial for authentication.</p>
{{else}}
<p>You're connecting ChatGPT to this xSocial account:</p>
<div class="scope"><strong>{{.Name}}</strong><br>{{.Email}}<br><br>
<strong>Permission requested</strong><br>
Manage brands, analytics, scheduled posts, publishing, inbox replies, SmartLinks, and reports.</div>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="transaction" value="{{.Transaction}}">
<input type="hidden" name="action" value="allow">
<button class="primary" type="submit">Allow access</button>
</form>
<form method="post" action="/oauth/authorize">
<input type="hidden" name="transaction" value="{{.Transaction}}">
<input type="hidden" name="action" value="deny">
<button class="secondary" type="submit">Deny</button>
</form>
{{end}}
</div></div></body></html>`))

func (s *Server) mcpAuthenticateUser(r *http.Request, email, password string) (bson.ObjectID, string, string, bool) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return bson.NilObjectID, "", "", false
	}
	ip := clientIP(r)
	if blocked, _ := s.checkAuthRate(ip); blocked {
		return bson.NilObjectID, "", "", false
	}
	result, err := s.authService.Login(r.Context(), email, password, "ChatGPT MCP OAuth", ip)
	if err != nil || result == nil {
		s.recordAuthFailure(ip)
		return bson.NilObjectID, "", "", false
	}
	s.recordAuthSuccess(ip)
	return result.User.ID, result.User.FullName, result.User.Email, true
}

func (s *Server) signMCPTransaction(tx mcpOAuthTransaction, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"mcp_oauth_tx": true, "response_type": tx.ResponseType, "client_id": tx.ClientID, "redirect_uri": tx.RedirectURI,
		"resource": tx.Resource, "scope": tx.Scope, "state": tx.State, "code_challenge": tx.CodeChallenge,
		"code_challenge_method": tx.CodeChallengeMethod, "iss": tx.Issuer, "phase": tx.Phase, "sub": tx.UserID,
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
		Issuer: get("iss"), Phase: get("phase"), UserID: get("sub"),
	}
	if tx.ResponseType != "code" || !validMCPClient(tx.ClientID) || !validMCPRedirect(tx.RedirectURI) ||
		tx.Resource != s.mcpOAuthResource(nil) || tx.CodeChallenge == "" || tx.CodeChallengeMethod != "S256" {
		return mcpOAuthTransaction{}, false
	}
	return tx, true
}

func renderMCPPage(w http.ResponseWriter, data mcpOAuthPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = mcpOAuthPageTemplate.Execute(w, data)
}

func (s *Server) mcpAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodGet {
		q := r.URL.Query()
		tx := mcpOAuthTransaction{
			ResponseType: q.Get("response_type"), ClientID: q.Get("client_id"), RedirectURI: q.Get("redirect_uri"),
			Resource: q.Get("resource"), Scope: strings.TrimSpace(q.Get("scope")), State: q.Get("state"),
			CodeChallenge: q.Get("code_challenge"), CodeChallengeMethod: q.Get("code_challenge_method"),
			Issuer: s.mcpOAuthIssuer(r), Phase: "login",
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
		renderMCPPage(w, mcpOAuthPageData{Title: "Connect xSocial", Transaction: rawTx})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	rawTx := strings.TrimSpace(r.FormValue("transaction"))
	tx, ok := s.parseMCPTransaction(rawTx)
	if !ok {
		http.Error(w, "authorization session expired or invalid", http.StatusBadRequest)
		return
	}
	switch r.FormValue("action") {
	case "login":
		if tx.Phase != "login" { http.Error(w, "invalid authorization step", http.StatusBadRequest); return }
		userID, name, email, authenticated := s.mcpAuthenticateUser(r, r.FormValue("email"), r.FormValue("password"))
		if !authenticated {
			renderMCPPage(w, mcpOAuthPageData{Title: "Connect xSocial", Transaction: rawTx, Error: "Invalid xSocial email or password."})
			return
		}
		tx.Phase, tx.UserID = "consent", userID.Hex()
		consentTx, err := s.signMCPTransaction(tx, 10*time.Minute)
		if err != nil {
			s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "server_error", "could not create consent session")
			return
		}
		renderMCPPage(w, mcpOAuthPageData{Title: "Authorize xSocial", Transaction: consentTx, Name: name, Email: email})
	case "deny":
		if tx.Phase != "consent" { http.Error(w, "invalid authorization step", http.StatusBadRequest); return }
		s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "access_denied", "user denied access")
	case "allow":
		if tx.Phase != "consent" { http.Error(w, "invalid authorization step", http.StatusBadRequest); return }
		if _, err := bson.ObjectIDFromHex(tx.UserID); err != nil {
			s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "server_error", "invalid authenticated user")
			return
		}
		jti, err := randomMCPString(24)
		if err != nil { s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "server_error", "could not create authorization code"); return }
		now := time.Now()
		claims := jwt.MapClaims{
			"mcp_code": true, "sub": tx.UserID, "client_id": tx.ClientID, "redirect_uri": tx.RedirectURI, "resource": tx.Resource,
			"code_challenge": tx.CodeChallenge, "code_challenge_method": "S256", "scope": tx.Scope, "jti": jti,
			"iat": now.Unix(), "exp": now.Add(2*time.Minute).Unix(), "iss": s.mcpOAuthIssuer(r),
		}
		code, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.cfg.JWTAccessSecret))
		if err != nil { s.mcpOAuthError(w, r, tx.RedirectURI, tx.State, "server_error", "could not sign authorization code"); return }
		u, _ := url.Parse(tx.RedirectURI)
		p := u.Query(); p.Set("code", code); if tx.State != "" { p.Set("state", tx.State) }; p.Set("iss", s.mcpOAuthIssuer(r)); u.RawQuery = p.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	default:
		http.Error(w, "invalid authorization action", http.StatusBadRequest)
	}
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

	refreshClaims := jwt.MapClaims{
		"mcp_refresh": true,
		"sub":         sub,
		"aud":         resource,
		"scope":       scopeFromMCPClaims(claims),
		"iss":         s.mcpOAuthIssuer(r),
		"iat":         now.Unix(),
		"exp":         now.Add(30 * 24 * time.Hour).Unix(),
	}
	refreshToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(s.cfg.JWTAccessSecret))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "server_error"})
		return
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

	sub, _ := claims["sub"].(string)
	if _, err := bson.ObjectIDFromHex(sub); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
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

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
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

func (s *Server) mcpAuthChallenge(w http.ResponseWriter, r *http.Request) {
	resourceMeta := s.mcpOAuthResource(r) + "/.well-known/oauth-protected-resource"
	challenge := fmt.Sprintf("Bearer resource_metadata=%q, scope=%q, error=%q, error_description=%q", resourceMeta, mcpOAuthScope, "invalid_token", "OAuth authentication required")
	w.Header().Set("WWW-Authenticate", challenge)
	writeJSON(w, http.StatusUnauthorized, map[string]any{
		"error":   "unauthorized",
		"message": "OAuth authentication required",
		"_meta": map[string]any{
			"mcp/www_authenticate": []string{challenge},
		},
	})
}

func mcpOAuthJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
