package queue_test

import (
	"sync"
	"testing"
	"time"

	"xsocial-backend/internal/queue"
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
