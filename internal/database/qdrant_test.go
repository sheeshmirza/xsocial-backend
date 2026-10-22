package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"xsocial-backend/internal/config"
)

func TestNewQdrantClient_EmptyURL(t *testing.T) {
	client, err := NewQdrantClient(&config.Config{QdrantURL: ""})
	if err != nil {
		t.Fatalf("expected nil error for empty QdrantURL, got %v", err)
	}
	if client != nil {
		t.Fatalf("expected nil client for empty QdrantURL, got %v", client)
	}
}

func TestQdrantClient_NilSafety(t *testing.T) {
	var client *QdrantClient
	ctx := context.Background()

	if err := client.Health(ctx); err == nil {
		t.Errorf("expected error for nil client Health(), got nil")
	}

	if err := client.EnsureCollection(ctx, "test", 128, "Cosine"); err == nil {
		t.Errorf("expected error for nil client EnsureCollection(), got nil")
	}

	if err := client.UpsertPoints(ctx, "test", nil); err == nil {
		t.Errorf("expected error for nil client UpsertPoints(), got nil")
	}

	if _, err := client.Search(ctx, "test", QdrantSearchRequest{}); err == nil {
		t.Errorf("expected error for nil client Search(), got nil")
	}

	if err := client.DeletePoints(ctx, "test", []string{"1"}); err == nil {
		t.Errorf("expected error for nil client DeletePoints(), got nil")
	}
}

func TestQdrantClient_Operations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/readyz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/collections/posts":
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"result":true,"status":"ok"}`))
				return
			}
		case "/collections/posts/points":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{"operation_id":1,"status":"completed"}}`))
		case "/collections/posts/points/search":
			resp := map[string]any{
				"result": []ScoredPoint{
					{
						ID:      "pt-1",
						Score:   0.95,
						Payload: map[string]any{"content": "Launch announcement"},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		QdrantURL: server.URL,
	}

	client, err := NewQdrantClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error creating qdrant client: %v", err)
	}

	ctx := context.Background()

	// 1. Health
	if err := client.Health(ctx); err != nil {
		t.Fatalf("health check failed: %v", err)
	}

	// 2. EnsureCollection
	if err := client.EnsureCollection(ctx, "posts", 128, "Cosine"); err != nil {
		t.Fatalf("ensure collection failed: %v", err)
	}

	// 3. UpsertPoints
	points := []QdrantPoint{
		{
			ID:      "pt-1",
			Vector:  []float32{0.1, 0.2, 0.3},
			Payload: map[string]any{"content": "Launch announcement"},
		},
	}
	if err := client.UpsertPoints(ctx, "posts", points); err != nil {
		t.Fatalf("upsert points failed: %v", err)
	}

	// 4. Search
	results, err := client.Search(ctx, "posts", QdrantSearchRequest{
		Vector: []float32{0.1, 0.2, 0.3},
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID != "pt-1" {
		t.Fatalf("expected ID pt-1, got %v", results[0].ID)
	}
}
