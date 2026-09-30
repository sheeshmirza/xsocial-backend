package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"xsocial-backend/internal/config"
)

func TestHealthCheck(t *testing.T) {
	srv := &Server{
		cfg: &config.Config{},
	}
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", res["status"])
	}
}

func TestMCPToolsList(t *testing.T) {
	secret := "test-secret-12345678901234567890"
	srv := &Server{
		cfg: &config.Config{
			JWTAccessSecret: secret,
		},
	}
	handler := srv.Handler()

	// Generate test JWT
	claims := jwt.MapClaims{
		"sub": "650c00000000000000000001",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	rpcBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      "test-1",
		"method":  "tools/list",
	}
	bodyBytes, _ := json.Marshal(rpcBody)

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	result, ok := res["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object in JSON-RPC response, got %v", res)
	}

	tools, ok := result["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Errorf("expected non-empty tools array, got %v", tools)
	}
}

func TestPublicTrackerPixel(t *testing.T) {
	srv := &Server{
		cfg: &config.Config{},
	}
	handler := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/v1/tracker/pixel.png", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "image/gif" {
		t.Errorf("expected content-type image/gif, got %s", rec.Header().Get("Content-Type"))
	}
}
