package utils_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"xsocial-backend/internal/utils"
)

func TestRedactString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Authorization: Bearer ya29.a0AfH6SMD_random_secret_token_123",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			input:    "Using key mc_live_9876543210abcdef in request",
			expected: "Using key [REDACTED_API_KEY] in request",
		},
		{
			input:    "Connected to mongodb://admin:SuperSecretPass@cluster0.mongodb.net:27017/xsocial",
			expected: "Connected to mongodb://[REDACTED_AUTH]@cluster0.mongodb.net:27017/xsocial",
		},
		{
			input:    "Connected to redis://default:SuperSecretRedisPass@redis-node:6379",
			expected: "Connected to redis://[REDACTED_AUTH]@redis-node:6379",
		},
	}

	for _, tt := range tests {
		got := utils.RedactString(tt.input)
		if got != tt.expected {
			t.Errorf("RedactString(%q) = %q; expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	t.Run("Masks mongo driver topology error", func(t *testing.T) {
		raw := "server selection error: server connection refused on cluster0.mongodb.net:27017"
		sanitized := utils.SanitizeErrorMessage(raw)
		if strings.Contains(sanitized, "cluster0") || strings.Contains(sanitized, "server selection error") {
			t.Errorf("driver details leaked: %s", sanitized)
		}
		if sanitized != "database connection temporarily unavailable; please try again later" {
			t.Errorf("unexpected sanitized error: %s", sanitized)
		}
	})

	t.Run("Masks credentials in error message", func(t *testing.T) {
		raw := "failed to connect using mc_live_secret123 and Bearer secret456"
		sanitized := utils.SanitizeErrorMessage(raw)
		if strings.Contains(sanitized, "secret123") || strings.Contains(sanitized, "secret456") {
			t.Errorf("credentials leaked: %s", sanitized)
		}
	})
}

func TestRedactingLogger(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := utils.NewRedactingLogger(buf, slog.LevelInfo, true)

	tc := utils.GenerateTraceContext("req-test-999")
	ctx := utils.ContextWithTrace(context.Background(), tc)

	logger.InfoContext(ctx, "Handling user login request",
		"password", "MyP@ssw0rd123!",
		"client_secret", "OAuthSecretKeyXYZ",
		"safe_field", "regular_value",
		"auth_header", "Bearer secret_access_token_xyz",
	)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log JSON: %v. Output: %s", err, buf.String())
	}

	if logEntry["password"] != "[REDACTED]" {
		t.Errorf("expected password to be [REDACTED], got %v", logEntry["password"])
	}
	if logEntry["client_secret"] != "[REDACTED]" {
		t.Errorf("expected client_secret to be [REDACTED], got %v", logEntry["client_secret"])
	}
	if logEntry["safe_field"] != "regular_value" {
		t.Errorf("expected safe_field regular_value, got %v", logEntry["safe_field"])
	}
	if !strings.Contains(logEntry["auth_header"].(string), "[REDACTED]") {
		t.Errorf("expected auth_header Bearer token to be redacted, got %v", logEntry["auth_header"])
	}
	if logEntry["trace_id"] != tc.TraceID {
		t.Errorf("expected trace_id %s, got %v", tc.TraceID, logEntry["trace_id"])
	}
	if logEntry["span_id"] != tc.SpanID {
		t.Errorf("expected span_id %s, got %v", tc.SpanID, logEntry["span_id"])
	}
	if logEntry["request_id"] != "req-test-999" {
		t.Errorf("expected request_id req-test-999, got %v", logEntry["request_id"])
	}
}
