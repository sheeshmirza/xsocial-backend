package common_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/utils"
)

func TestHTTPClientTracePropagation(t *testing.T) {
	var capturedTraceparent string
	var capturedRequestID string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedTraceparent = r.Header.Get("traceparent")
		capturedRequestID = r.Header.Get("X-Request-ID")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	client := common.NewHTTPClient(5 * time.Second)

	parentTC := utils.GenerateTraceContext("test-client-req-id")
	ctx := utils.ContextWithTrace(context.Background(), parentTC)

	_, status, err := client.Get(ctx, ts.URL, nil, nil)
	if err != nil {
		t.Fatalf("client.Get failed: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}

	if capturedRequestID != "test-client-req-id" {
		t.Errorf("expected X-Request-ID 'test-client-req-id', got %q", capturedRequestID)
	}

	if capturedTraceparent == "" {
		t.Fatal("expected traceparent header to be present on outbound request")
	}

	childTC, ok := utils.ParseTraceparent(capturedTraceparent)
	if !ok {
		t.Fatalf("outbound traceparent header was not valid W3C format: %s", capturedTraceparent)
	}
	if childTC.TraceID != parentTC.TraceID {
		t.Errorf("child TraceID %s does not match parent %s", childTC.TraceID, parentTC.TraceID)
	}
	if childTC.SpanID == parentTC.SpanID {
		t.Errorf("child SpanID should be a new distinct span ID, got %s", childTC.SpanID)
	}
}
