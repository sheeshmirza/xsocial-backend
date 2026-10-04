package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"xsocial-backend/internal/utils"
)

type apiErrorResponse struct {
	Success   bool   `json:"success"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
	Timestamp string `json:"timestamp"`
}

func TestDistributedTracingIngress(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	t.Run("Auto-generates traceparent and request ID when missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		reqID := rec.Header().Get("X-Request-ID")
		if reqID == "" {
			t.Fatal("expected X-Request-ID to be generated in response")
		}

		traceparent := rec.Header().Get("traceparent")
		if traceparent == "" {
			t.Fatal("expected traceparent to be generated in response")
		}

		tc, ok := utils.ParseTraceparent(traceparent)
		if !ok {
			t.Fatalf("generated traceparent %q is not valid W3C format", traceparent)
		}
		if len(tc.TraceID) != 32 {
			t.Errorf("expected 32 hex chars for TraceID, got %d", len(tc.TraceID))
		}
	})

	t.Run("Preserves incoming traceparent trace ID and custom request ID", func(t *testing.T) {
		incomingTrace := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		customReqID := "inbound-trace-test-uuid-42"

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("traceparent", incomingTrace)
		req.Header.Set("X-Request-ID", customReqID)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		if gotReqID := rec.Header().Get("X-Request-ID"); gotReqID != customReqID {
			t.Errorf("expected echoed X-Request-ID %q, got %q", customReqID, gotReqID)
		}

		outTrace := rec.Header().Get("traceparent")
		if outTrace == "" {
			t.Fatal("expected response traceparent header")
		}

		tc, ok := utils.ParseTraceparent(outTrace)
		if !ok {
			t.Fatalf("invalid response traceparent: %s", outTrace)
		}
		if tc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Errorf("expected TraceID to be preserved from incoming trace, got %s", tc.TraceID)
		}
	})
}

func TestStandardizedErrorEnvelopes(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	t.Run("401 Missing Authorization Envelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
		req.Header.Set("X-Request-ID", "err-test-401")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}

		var errResp apiErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to decode error json: %v", err)
		}

		if errResp.Success {
			t.Error("expected success: false")
		}
		if errResp.Error == "" || errResp.Message == "" {
			t.Errorf("expected error and message to be populated, got error=%q message=%q", errResp.Error, errResp.Message)
		}
		if errResp.RequestID != "err-test-401" {
			t.Errorf("expected requestId 'err-test-401', got %q", errResp.RequestID)
		}
		if errResp.Timestamp == "" {
			t.Error("expected non-empty timestamp")
		}
	})

	t.Run("401 Invalid API Key Format Envelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer mc_live_invalid_fake_key_999")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}

		var errResp apiErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to decode error json: %v", err)
		}
		if errResp.Success {
			t.Error("expected success: false")
		}
		if !strings.Contains(strings.ToLower(errResp.Message), "api key") {
			t.Errorf("expected API key error message, got %q", errResp.Message)
		}
		if rec.Header().Get("X-Request-ID") == "" {
			t.Error("expected response X-Request-ID")
		}
	})

	t.Run("400 Bad Request JSON Payload Envelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/signup", bytes.NewBufferString(`{malformed-json`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var errResp apiErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to decode error json: %v", err)
		}
		if errResp.Success {
			t.Error("expected success: false")
		}
		if errResp.Error == "" {
			t.Error("expected non-empty error field")
		}
		if errResp.RequestID == "" {
			t.Error("expected non-empty requestId")
		}
	})
}

func TestPostReviewAndApprovalWorkflow(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	t.Run("GET Review by invalid token returns 404 envelope", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/posts/review/nonexistent-token-12345", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}

		var errResp apiErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to parse json response: %v", err)
		}

		if errResp.Success {
			t.Error("expected success: false")
		}
		if !strings.Contains(strings.ToLower(errResp.Message), "not found") &&
			!strings.Contains(strings.ToLower(errResp.Message), "invalid") {
			t.Errorf("expected review link invalid or not found, got %q", errResp.Message)
		}
	})

	t.Run("POST Review Action rejects invalid action type", func(t *testing.T) {
		payload := `{"action":"invalid_action","reviewerName":"Bob"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/posts/review/some-token/action", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}

		var errResp apiErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to parse json: %v", err)
		}
		if errResp.Success {
			t.Error("expected success: false")
		}
	})

	t.Run("POST Review Action rejects reviewerName exceeding 100 chars", func(t *testing.T) {
		payload := `{"action":"approve","reviewerName":"` + strings.Repeat("A", 101) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/posts/review/some-token/action", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestMCPOAuthAndJSONRPCWorkflows(t *testing.T) {
	srv := createTestServer()
	handler := srv.Handler()

	t.Run("MCP Ping", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":10,"method":"ping","params":{}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp struct {
			Jsonrpc string         `json:"jsonrpc"`
			ID      int            `json:"id"`
			Result  map[string]any `json:"result"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse ping response: %v", err)
		}
		if resp.Jsonrpc != "2.0" || resp.ID != 10 {
			t.Errorf("unexpected jsonrpc envelope: %+v", resp)
		}
	})

	t.Run("MCP Unknown Method returns error code -32601", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":99,"method":"nonexistent/method","params":{}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var resp struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse json-rpc error: %v", err)
		}
		if resp.Error.Code != -32601 {
			t.Errorf("expected error code -32601 (Method not found), got %d", resp.Error.Code)
		}
	})

	t.Run("MCP Tool Call without auth triggers OAuth WWW-Authenticate challenge", func(t *testing.T) {
		payload := `{"jsonrpc":"2.0","id":100,"method":"tools/call","params":{"name":"list_posts","arguments":{}}}`
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized for unlinked tool call, got %d", rec.Code)
		}

		authHeader := rec.Header().Get("WWW-Authenticate")
		if !strings.Contains(authHeader, "Bearer") || !strings.Contains(authHeader, "resource_metadata") {
			t.Errorf("expected WWW-Authenticate header to contain Bearer challenge, got %q", authHeader)
		}

		var resp struct {
			Result struct {
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if !resp.Result.IsError {
			t.Error("expected result.isError to be true in challenge response")
		}
	})
}
