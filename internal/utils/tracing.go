package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// traceparent format according to W3C Trace Context spec:
// version(2 hex) - trace_id(32 hex) - parent_id/span_id(16 hex) - trace_flags(2 hex)
var traceparentRegex = regexp.MustCompile(`^([0-9a-f]{2})-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)

type traceContextKey struct{}
type requestIDContextKey struct{}

// TraceContext encapsulates W3C Trace Context and request identification.
type TraceContext struct {
	TraceID    string `json:"traceId"`
	SpanID     string `json:"spanId"`
	ParentID   string `json:"parentId,omitempty"`
	TraceFlags string `json:"traceFlags"`
	RequestID  string `json:"requestId"`
}

// Traceparent formats the trace context into the standard W3C traceparent header.
func (t TraceContext) Traceparent() string {
	if t.TraceID == "" || t.SpanID == "" {
		return ""
	}
	flags := t.TraceFlags
	if flags == "" {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", t.TraceID, t.SpanID, flags)
}

// NewChildSpan creates a child trace context with a newly generated span ID,
// pointing ParentID to the current SpanID and retaining the TraceID, TraceFlags, and RequestID.
func (t TraceContext) NewChildSpan() TraceContext {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	childSpanID := hex.EncodeToString(b)
	flags := t.TraceFlags
	if flags == "" {
		flags = "01"
	}
	return TraceContext{
		TraceID:    t.TraceID,
		SpanID:     childSpanID,
		ParentID:   t.SpanID,
		TraceFlags: flags,
		RequestID:  t.RequestID,
	}
}

// ParseTraceparent validates and extracts components from a W3C traceparent header.
// It returns false if the header is malformed, has an unsupported version, or consists of all zeros.
func ParseTraceparent(raw string) (TraceContext, bool) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	match := traceparentRegex.FindStringSubmatch(raw)
	if match == nil {
		return TraceContext{}, false
	}

	version := match[1]
	traceID := match[2]
	spanID := match[3]
	flags := match[4]

	// Disallow version FF or all-zero trace_id/span_id as per W3C specification
	if version == "ff" || traceID == "00000000000000000000000000000000" || spanID == "0000000000000000" {
		return TraceContext{}, false
	}

	return TraceContext{
		TraceID:    traceID,
		SpanID:     spanID,
		ParentID:   spanID,
		TraceFlags: flags,
	}, true
}

// GenerateTraceContext generates a cryptographically random W3C TraceContext and RequestID.
func GenerateTraceContext(reqID string) TraceContext {
	if reqID == "" {
		reqID = uuid.NewString()
	}

	traceBytes := make([]byte, 16)
	_, _ = rand.Read(traceBytes)
	traceID := hex.EncodeToString(traceBytes)

	spanBytes := make([]byte, 8)
	_, _ = rand.Read(spanBytes)
	spanID := hex.EncodeToString(spanBytes)

	return TraceContext{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: "01",
		RequestID:  reqID,
	}
}

// ContextWithTrace returns a child context carrying the TraceContext.
func ContextWithTrace(ctx context.Context, tc TraceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, traceContextKey{}, tc)
	if tc.RequestID != "" {
		ctx = context.WithValue(ctx, requestIDContextKey{}, tc.RequestID)
	}
	return ctx
}

// ContextWithTraceparent parses or generates a TraceContext and attaches it to ctx.
func ContextWithTraceparent(ctx context.Context, rawTraceparent, reqID string) context.Context {
	tc, ok := ParseTraceparent(rawTraceparent)
	if !ok {
		tc = GenerateTraceContext(reqID)
	} else {
		if reqID == "" {
			reqID = uuid.NewString()
		}
		tc.RequestID = reqID
	}
	return ContextWithTrace(ctx, tc)
}

// TraceFromContext retrieves the TraceContext from ctx if present.
func TraceFromContext(ctx context.Context) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	tc, ok := ctx.Value(traceContextKey{}).(TraceContext)
	return tc, ok
}

// TraceIDFromContext retrieves the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	if tc, ok := TraceFromContext(ctx); ok {
		return tc.TraceID
	}
	return ""
}

// SpanIDFromContext retrieves the span ID from context.
func SpanIDFromContext(ctx context.Context) string {
	if tc, ok := TraceFromContext(ctx); ok {
		return tc.SpanID
	}
	return ""
}

// TraceparentFromContext retrieves the formatted traceparent header string from context.
func TraceparentFromContext(ctx context.Context) string {
	if tc, ok := TraceFromContext(ctx); ok {
		return tc.Traceparent()
	}
	return ""
}

// RequestIDFromContext retrieves the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if tc, ok := TraceFromContext(ctx); ok && tc.RequestID != "" {
		return tc.RequestID
	}
	if id, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return id
	}
	return ""
}

// InjectTraceHeaders injects W3C traceparent (with a newly derived child span)
// and X-Request-ID headers into an outgoing HTTP request.
func InjectTraceHeaders(ctx context.Context, req *http.Request) {
	if req == nil {
		return
	}
	if tc, ok := TraceFromContext(ctx); ok && tc.TraceID != "" {
		child := tc.NewChildSpan()
		req.Header.Set("traceparent", child.Traceparent())
		if tc.RequestID != "" {
			req.Header.Set("X-Request-ID", tc.RequestID)
		}
	} else if reqID := RequestIDFromContext(ctx); reqID != "" {
		req.Header.Set("X-Request-ID", reqID)
	}
}
