package utils_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"xsocial-backend/internal/utils"
)

func TestParseTraceparent(t *testing.T) {
	t.Run("Valid W3C header", func(t *testing.T) {
		header := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
		tc, ok := utils.ParseTraceparent(header)
		if !ok {
			t.Fatal("expected valid traceparent parse")
		}
		if tc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
			t.Errorf("unexpected TraceID: %s", tc.TraceID)
		}
		if tc.SpanID != "00f067aa0ba902b7" {
			t.Errorf("unexpected SpanID: %s", tc.SpanID)
		}
		if tc.TraceFlags != "01" {
			t.Errorf("unexpected TraceFlags: %s", tc.TraceFlags)
		}
		if tc.Traceparent() != header {
			t.Errorf("expected Traceparent() %q, got %q", header, tc.Traceparent())
		}
	})

	t.Run("Invalid formats", func(t *testing.T) {
		invalids := []string{
			"",
			"invalid",
			"00-short-span-01",
			"ff-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", // ff version forbidden
			"00-00000000000000000000000000000000-00f067aa0ba902b7-01", // all-zero trace forbidden
			"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01", // all-zero span forbidden
		}
		for _, inv := range invalids {
			if _, ok := utils.ParseTraceparent(inv); ok {
				t.Errorf("expected parse failure for %q", inv)
			}
		}
	})
}

func TestGenerateTraceContext(t *testing.T) {
	tc := utils.GenerateTraceContext("req-test-123")
	if len(tc.TraceID) != 32 {
		t.Errorf("expected 32 hex chars for TraceID, got %d (%s)", len(tc.TraceID), tc.TraceID)
	}
	if len(tc.SpanID) != 16 {
		t.Errorf("expected 16 hex chars for SpanID, got %d (%s)", len(tc.SpanID), tc.SpanID)
	}
	if tc.RequestID != "req-test-123" {
		t.Errorf("expected req-test-123, got %s", tc.RequestID)
	}

	child := tc.NewChildSpan()
	if child.TraceID != tc.TraceID {
		t.Errorf("expected child to have same TraceID, got %s", child.TraceID)
	}
	if child.ParentID != tc.SpanID {
		t.Errorf("expected child ParentID to equal parent SpanID %s, got %s", tc.SpanID, child.ParentID)
	}
	if child.SpanID == tc.SpanID {
		t.Errorf("expected child SpanID to differ from parent SpanID")
	}
}

func TestTraceContextPropagation(t *testing.T) {
	ctx := context.Background()
	tc := utils.GenerateTraceContext("custom-req-id")
	ctx = utils.ContextWithTrace(ctx, tc)

	if gotID := utils.TraceIDFromContext(ctx); gotID != tc.TraceID {
		t.Errorf("expected TraceID %s, got %s", tc.TraceID, gotID)
	}
	if gotReqID := utils.RequestIDFromContext(ctx); gotReqID != "custom-req-id" {
		t.Errorf("expected RequestID custom-req-id, got %s", gotReqID)
	}
	if gotSpanID := utils.SpanIDFromContext(ctx); gotSpanID != tc.SpanID {
		t.Errorf("expected SpanID %s, got %s", tc.SpanID, gotSpanID)
	}
	if gotParent := utils.TraceparentFromContext(ctx); gotParent != tc.Traceparent() {
		t.Errorf("expected Traceparent %s, got %s", tc.Traceparent(), gotParent)
	}
}

func TestInjectTraceHeaders(t *testing.T) {
	tc := utils.GenerateTraceContext("outbound-req-456")
	ctx := utils.ContextWithTrace(context.Background(), tc)

	req := httptest.NewRequest(http.MethodGet, "https://api.linkedin.com/v2/me", nil)
	utils.InjectTraceHeaders(ctx, req)

	traceHeader := req.Header.Get("traceparent")
	if traceHeader == "" {
		t.Fatal("expected traceparent header to be injected")
	}
	childTC, ok := utils.ParseTraceparent(traceHeader)
	if !ok {
		t.Fatalf("expected injected traceparent to be valid, got %s", traceHeader)
	}
	if childTC.TraceID != tc.TraceID {
		t.Errorf("expected child trace ID to match parent trace ID: %s vs %s", childTC.TraceID, tc.TraceID)
	}
	if childTC.SpanID == tc.SpanID {
		t.Errorf("expected child span ID to be newly generated, was identical to %s", tc.SpanID)
	}
	if req.Header.Get("X-Request-ID") != "outbound-req-456" {
		t.Errorf("expected X-Request-ID outbound-req-456, got %s", req.Header.Get("X-Request-ID"))
	}
}
