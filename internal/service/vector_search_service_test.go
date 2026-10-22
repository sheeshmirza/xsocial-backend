package service

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

func TestGenerateDeterministicEmbedding(t *testing.T) {
	vec1 := GenerateDeterministicEmbedding("Artificial Intelligence and Social Media", 128)
	if len(vec1) != 128 {
		t.Fatalf("expected 128 dimensions, got %d", len(vec1))
	}

	// Determinism check: same input must yield identical vector
	vec2 := GenerateDeterministicEmbedding("Artificial Intelligence and Social Media", 128)
	for i := range vec1 {
		if vec1[i] != vec2[i] {
			t.Errorf("expected deterministic embedding: index %d differed (%v vs %v)", i, vec1[i], vec2[i])
		}
	}

	// Empty text check
	emptyVec := GenerateDeterministicEmbedding("", 128)
	if len(emptyVec) != 128 {
		t.Fatalf("expected 128 dimensions for empty text, got %d", len(emptyVec))
	}
}

func TestVectorSearchService_NilSafety(t *testing.T) {
	var svc *VectorSearchService
	ctx := context.Background()

	if err := svc.IndexPost(ctx, nil); err != nil {
		t.Errorf("expected nil error on nil service IndexPost, got %v", err)
	}

	if err := svc.DeletePostVector(ctx, "id"); err != nil {
		t.Errorf("expected nil error on nil service DeletePostVector, got %v", err)
	}

	results, err := svc.SearchSimilarPosts(ctx, "brand", "query", 5)
	if err != nil {
		t.Errorf("expected nil error on nil service SearchSimilarPosts, got %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	// Non-nil svc with nil Qdrant
	svc = NewVectorSearchService(nil)
	brandID := bson.NewObjectID()
	post := &models.Post{
		ID:        bson.NewObjectID(),
		BrandID:   &brandID,
		Caption:   "Testing vector search nil safety",
		Status:    "draft",
		CreatedAt: time.Now(),
	}
	if err := svc.IndexPost(ctx, post); err != nil {
		t.Errorf("expected nil error on IndexPost with nil qdrant, got %v", err)
	}
	if err := svc.DeletePostVector(ctx, post.ID.Hex()); err != nil {
		t.Errorf("expected nil error on DeletePostVector with nil qdrant, got %v", err)
	}
	res, err := svc.SearchSimilarPosts(ctx, post.BrandID.Hex(), "search", 5)
	if err != nil || len(res) != 0 {
		t.Errorf("expected empty results without error, got res=%v, err=%v", res, err)
	}
}
