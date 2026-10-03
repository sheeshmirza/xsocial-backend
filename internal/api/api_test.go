package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xsocial-backend/internal/api"
	"xsocial-backend/internal/config"
)

func createTestServer() *api.Server {
	cfg := &config.Config{
		Port:            5000,
		ApiPrefix:       "/v1",
		PublicBaseURL:   "http://localhost:5000",
		NodeEnv:         "test",
		JWTAccessSecret: "test_jwt_access_secret_for_unit_tests_32chars_long",
	}
	return api.NewWithDeps(api.ServerDeps{
		Config: cfg,
	})
}

func TestHealthEndpoints(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	for _, endpoint := range []string{"/healthz", "/health"} {
		req := httptest.NewRequest(http.MethodGet, endpoint, nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", endpoint, rec.Code)
		}

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to parse json response: %v", err)
		}

		if body["status"] != "healthy" {
			t.Errorf("expected status 'healthy', got %v", body["status"])
		}
	}
}

func TestOAuthWellKnownEndpoints(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	t.Run("Protected Resource Metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["resource"] != "http://localhost:5000" {
			t.Errorf("expected resource 'http://localhost:5000', got %v", body["resource"])
		}
	})

	t.Run("Authorization Server Metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["authorization_endpoint"] != "http://localhost:5000/oauth/authorize" {
			t.Errorf("expected auth endpoint, got %v", body["authorization_endpoint"])
		}
	})
}

func TestMCPEndpoint(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	t.Run("MCP Initialize", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body struct {
			Result struct {
				ProtocolVersion string `json:"protocolVersion"`
				ServerInfo      struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"serverInfo"`
			} `json:"result"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode initialize json: %v", err)
		}

		if body.Result.ServerInfo.Name != "xsocial-mcp" {
			t.Errorf("expected server name 'xsocial-mcp', got %q", body.Result.ServerInfo.Name)
		}
		if body.Result.ServerInfo.Version != "1.2.0" {
			t.Errorf("expected version '1.2.0', got %q", body.Result.ServerInfo.Version)
		}
	})

	t.Run("MCP Tools List", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var body struct {
			Result struct {
				Tools []struct {
					Name string `json:"name"`
				} `json:"tools"`
			} `json:"result"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode tools/list json: %v", err)
		}

		if len(body.Result.Tools) != 12 {
			t.Errorf("expected 12 MCP tools, got %d", len(body.Result.Tools))
		}
	})
}

func TestSignupValidation(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	t.Run("Reject short password", func(t *testing.T) {
		payload := `{"email":"test@example.com","password":"short"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Reject invalid email format", func(t *testing.T) {
		payload := `{"email":"invalid-email-address","password":"ValidPassword123!"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rec.Code)
		}
	})
}
