package utils

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

var (
	bearerRegex      = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9\-._~+/]+=*`)
	apiKeyRegex      = regexp.MustCompile(`(?i)\b(?:mc_live_|xs_live_)[a-zA-Z0-9_\-]+\b`)
	mongoURIRegex    = regexp.MustCompile(`(?i)mongodb(\+srv)?:\/\/[^@\s]+@`)
	redisURIRegex    = regexp.MustCompile(`(?i)redis:\/\/[^@\s]+@`)
	jwtRegex         = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{10,}\.eyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\b`)
	mongoDriverRegex = regexp.MustCompile(`(?i)(server selection error|topology is closed|connection\(\) error occured during connection handshake|no reachable servers)`)
)

var sensitiveKeySubstrings = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"access_token",
	"refresh_token",
	"authorization",
	"api_key",
	"apikey",
	"private_key",
	"client_secret",
	"signing_secret",
	"jwt",
}

// RedactString sanitizes tokens, credentials, connection strings, and API keys from a string.
func RedactString(s string) string {
	if s == "" {
		return ""
	}
	s = bearerRegex.ReplaceAllString(s, "Bearer [REDACTED]")
	s = apiKeyRegex.ReplaceAllString(s, "[REDACTED_API_KEY]")
	s = mongoURIRegex.ReplaceAllString(s, "mongodb$1://[REDACTED_AUTH]@")
	s = redisURIRegex.ReplaceAllString(s, "redis://[REDACTED_AUTH]@")
	s = jwtRegex.ReplaceAllString(s, "[REDACTED_JWT]")
	if strings.Contains(s, "http://") || strings.Contains(s, "https://") {
		s = MaskURL(s)
	}
	return s
}

// isSensitiveKey checks if an attribute key matches sensitive credential naming patterns.
func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	for _, pattern := range sensitiveKeySubstrings {
		if strings.Contains(k, pattern) {
			return true
		}
	}
	return false
}

// redactAttr sanitizes a single slog.Attr, masking sensitive keys and redacting string values.
func redactAttr(a slog.Attr) slog.Attr {
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, "[REDACTED]")
	}
	switch a.Value.Kind() {
	case slog.KindString:
		return slog.String(a.Key, RedactString(a.Value.String()))
	case slog.KindGroup:
		attrs := a.Value.Group()
		redacted := make([]slog.Attr, len(attrs))
		for i, sub := range attrs {
			redacted[i] = redactAttr(sub)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	default:
		return a
	}
}

// SanitizeErrorMessage cleans an error message for safe client delivery,
// shielding internal database driver errors and masking credentials.
func SanitizeErrorMessage(msg string) string {
	if msg == "" {
		return "an unexpected error occurred"
	}
	if mongoDriverRegex.MatchString(msg) {
		return "database connection temporarily unavailable; please try again later"
	}
	return RedactString(msg)
}

// RedactingHandler is an slog.Handler that extracts distributed tracing context
// and masks credentials in log messages and attributes.
type RedactingHandler struct {
	next slog.Handler
}

// NewRedactingHandler creates an slog.Handler wrapping the given handler with credential redaction
// and distributed trace context injection.
func NewRedactingHandler(next slog.Handler) *RedactingHandler {
	return &RedactingHandler{next: next}
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, r slog.Record) error {
	// 1. Sanitize message
	r.Message = RedactString(r.Message)

	// 2. Extract tracing & request IDs from context if available
	hasTraceID := false
	hasSpanID := false
	hasReqID := false

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "trace_id":
			hasTraceID = true
		case "span_id":
			hasSpanID = true
		case "request_id":
			hasReqID = true
		}
		return true
	})

	var extraAttrs []slog.Attr
	if ctx != nil {
		if tc, ok := TraceFromContext(ctx); ok {
			if !hasTraceID && tc.TraceID != "" {
				extraAttrs = append(extraAttrs, slog.String("trace_id", tc.TraceID))
			}
			if !hasSpanID && tc.SpanID != "" {
				extraAttrs = append(extraAttrs, slog.String("span_id", tc.SpanID))
			}
			if !hasReqID && tc.RequestID != "" {
				extraAttrs = append(extraAttrs, slog.String("request_id", tc.RequestID))
			}
		} else if reqID := RequestIDFromContext(ctx); reqID != "" && !hasReqID {
			extraAttrs = append(extraAttrs, slog.String("request_id", reqID))
		}
	}

	// 3. Build new record with redacted attributes
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		newRecord.AddAttrs(redactAttr(a))
		return true
	})
	if len(extraAttrs) > 0 {
		newRecord.AddAttrs(extraAttrs...)
	}

	return h.next.Handle(ctx, newRecord)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	redacted := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		redacted[i] = redactAttr(a)
	}
	return &RedactingHandler{next: h.next.WithAttrs(redacted)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{next: h.next.WithGroup(name)}
}

// NewRedactingLogger creates an *slog.Logger configured with RedactingHandler.
func NewRedactingLogger(w io.Writer, level slog.Level, isJSON bool) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}
	opts := &slog.HandlerOptions{
		Level: level,
	}
	var baseHandler slog.Handler
	if isJSON {
		baseHandler = slog.NewJSONHandler(w, opts)
	} else {
		baseHandler = slog.NewTextHandler(w, opts)
	}
	return slog.New(NewRedactingHandler(baseHandler))
}
