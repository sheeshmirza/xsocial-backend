package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"xsocial-backend/internal/queue"
	"xsocial-backend/internal/utils"
)

func TestPostQueue_CloseIdempotent(t *testing.T) {
	pq := queue.NewPostQueue(nil, func(name string, payload []byte) error {
		return nil
	})

	// Closing once
	pq.Close()

	// Closing again should not panic (protected by sync.Once)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on second Close: %v", r)
		}
	}()
	pq.Close()
	pq.Close()
}

func TestPostQueue_MemoryTimerPanicRecovery(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	// Handler intentionally panics
	pq := queue.NewPostQueue(nil, func(name string, payload []byte) error {
		defer wg.Done()
		panic("simulated fatal worker panic")
	})
	defer pq.Close()

	err := pq.AddSchedulePublishJob("test-post-panic-123", time.Now().Add(10*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error adding job: %v", err)
	}

	select {
	case <-time.After(200 * time.Millisecond):
		// If timer fired and panic was recovered, test passes
	}

	// Verify queue is still functional
	_ = pq.RemoveSchedulePublishJob("test-post-panic-123")
}

type mockExecutor struct {
	mu            sync.Mutex
	capturedCtx   context.Context
	capturedPostID string
}

func (m *mockExecutor) PublishScheduledPost(ctx context.Context, postID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capturedCtx = ctx
	m.capturedPostID = postID
	return nil
}

func (m *mockExecutor) ExpirePublishedPlatformPost(ctx context.Context, payload queue.ExpiryJobData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capturedCtx = ctx
	return nil
}

func TestPostQueue_TraceContextPropagation(t *testing.T) {
	mockExec := &mockExecutor{}
	dispatcher := queue.CreateJobDispatcher(mockExec)

	pq := queue.NewPostQueue(nil, dispatcher)
	defer pq.Close()

	parentTC := utils.GenerateTraceContext("queue-req-777")
	ctx := utils.ContextWithTrace(context.Background(), parentTC)

	err := pq.AddSchedulePublishJobCtx(ctx, "post-trace-abc", time.Now().Add(10*time.Millisecond))
	if err != nil {
		t.Fatalf("unexpected error adding job with context: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	mockExec.mu.Lock()
	defer mockExec.mu.Unlock()

	if mockExec.capturedPostID != "post-trace-abc" {
		t.Fatalf("expected executed post ID 'post-trace-abc', got %q", mockExec.capturedPostID)
	}
	if mockExec.capturedCtx == nil {
		t.Fatal("expected captured context to be non-nil")
	}

	capturedTraceID := utils.TraceIDFromContext(mockExec.capturedCtx)
	if capturedTraceID != parentTC.TraceID {
		t.Errorf("expected worker context to preserve TraceID %s, got %s", parentTC.TraceID, capturedTraceID)
	}
	capturedReqID := utils.RequestIDFromContext(mockExec.capturedCtx)
	if capturedReqID != "queue-req-777" {
		t.Errorf("expected worker context to preserve RequestID 'queue-req-777', got %s", capturedReqID)
	}
}
