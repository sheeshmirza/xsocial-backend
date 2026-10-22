package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"xsocial-backend/internal/database"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/utils"
)

const (
	DefaultVectorDimension = 128
	CollectionSocialPosts  = "social_posts"
)

// SemanticPostResult represents a matched post with semantic similarity score.
type SemanticPostResult struct {
	PostID    string    `json:"postId"`
	BrandID   string    `json:"brandId"`
	Content   string    `json:"content"`
	Platform  string    `json:"platform"`
	Status    string    `json:"status"`
	Score     float32   `json:"score"`
	CreatedAt time.Time `json:"createdAt"`
}

// VectorSearchService handles post vectorization, indexing, and similarity search via Qdrant.
type VectorSearchService struct {
	qdrant *database.QdrantClient
	log    *slog.Logger
}

// NewVectorSearchService initializes the vector search service and ensures Qdrant collections.
func NewVectorSearchService(qdrant *database.QdrantClient) *VectorSearchService {
	logger := slog.Default().With("component", "vector_search")
	svc := &VectorSearchService{
		qdrant: qdrant,
		log:    logger,
	}

	if qdrant != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = qdrant.EnsureCollection(ctx, CollectionSocialPosts, DefaultVectorDimension, "Cosine")
	}

	return svc
}

// GenerateDeterministicEmbedding produces a normalized 128-dimensional dense vector
// from text using multi-hash n-gram projection, ensuring high-speed local vectorization.
func GenerateDeterministicEmbedding(text string, dim int) []float32 {
	if dim <= 0 {
		dim = DefaultVectorDimension
	}
	vec := make([]float32, dim)
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	if len(tokens) == 0 {
		return vec
	}

	for _, token := range tokens {
		h := sha256.Sum256([]byte(token))
		for i := 0; i < len(h)-1; i += 2 {
			idx := (int(h[i])<<8 | int(h[i+1])) % dim
			weight := float32((int(h[i])%100)-50) / 50.0
			vec[idx] += weight
		}
	}

	// L2 Normalize
	var sumSquares float64
	for _, v := range vec {
		sumSquares += float64(v * v)
	}
	if sumSquares > 0 {
		norm := float32(math.Sqrt(sumSquares))
		for i := range vec {
			vec[i] /= norm
		}
	}

	return vec
}

// IndexPost indexes a post's content and metadata into the Qdrant vector database.
func (s *VectorSearchService) IndexPost(ctx context.Context, post *models.Post) error {
	if s == nil || s.qdrant == nil || post == nil {
		return nil
	}

	content := strings.TrimSpace(post.Caption)
	if content == "" {
		content = strings.TrimSpace(post.Title)
	}
	if content == "" {
		return nil
	}

	brandID := ""
	if post.BrandID != nil {
		brandID = post.BrandID.Hex()
	}

	vec := GenerateDeterministicEmbedding(content, DefaultVectorDimension)
	point := database.QdrantPoint{
		ID:     post.ID.Hex(),
		Vector: vec,
		Payload: map[string]any{
			"post_id":    post.ID.Hex(),
			"brand_id":   brandID,
			"content":    utils.SanitizeString(content),
			"status":     post.Status,
			"created_at": post.CreatedAt.Format(time.RFC3339),
		},
	}

	return s.qdrant.UpsertPoints(ctx, CollectionSocialPosts, []database.QdrantPoint{point})
}

// DeletePostVector removes a post from Qdrant vector collection.
func (s *VectorSearchService) DeletePostVector(ctx context.Context, postID string) error {
	if s == nil || s.qdrant == nil || postID == "" {
		return nil
	}
	return s.qdrant.DeletePoints(ctx, CollectionSocialPosts, []string{postID})
}

// SearchSimilarPosts queries Qdrant for semantic similarity matches filtered by brand ID.
func (s *VectorSearchService) SearchSimilarPosts(ctx context.Context, brandID, queryText string, limit int) ([]SemanticPostResult, error) {
	if s == nil || s.qdrant == nil {
		return []SemanticPostResult{}, nil
	}

	if limit <= 0 {
		limit = 10
	}

	queryVec := GenerateDeterministicEmbedding(queryText, DefaultVectorDimension)
	req := database.QdrantSearchRequest{
		Vector: queryVec,
		Limit:  limit,
	}

	if brandID != "" {
		req.Filter = map[string]any{
			"must": []map[string]any{
				{
					"key": "brand_id",
					"match": map[string]any{
						"value": brandID,
					},
				},
			},
		}
	}

	points, err := s.qdrant.Search(ctx, CollectionSocialPosts, req)
	if err != nil {
		return nil, fmt.Errorf("semantic search query failed: %w", err)
	}

	results := make([]SemanticPostResult, 0, len(points))
	for _, p := range points {
		res := SemanticPostResult{
			PostID: fmt.Sprintf("%v", p.Payload["post_id"]),
			BrandID: fmt.Sprintf("%v", p.Payload["brand_id"]),
			Content: fmt.Sprintf("%v", p.Payload["content"]),
			Status:  fmt.Sprintf("%v", p.Payload["status"]),
			Score:   p.Score,
		}
		if timeStr, ok := p.Payload["created_at"].(string); ok {
			res.CreatedAt, _ = time.Parse(time.RFC3339, timeStr)
		}
		results = append(results, res)
	}

	return results, nil
}
