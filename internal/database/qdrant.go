package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/utils"
)

// QdrantPoint represents a vector data point with an identifier, embedding values, and metadata payload.
type QdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload,omitempty"`
}

// ScoredPoint represents a similarity match returned from a Qdrant vector query.
type ScoredPoint struct {
	ID      any            `json:"id"`
	Version int            `json:"version"`
	Score   float32        `json:"score"`
	Payload map[string]any `json:"payload,omitempty"`
}

// QdrantSearchRequest encapsulates the search parameters for vector similarity queries.
type QdrantSearchRequest struct {
	Vector      []float32      `json:"vector"`
	Limit       int            `json:"limit"`
	WithPayload bool           `json:"with_payload"`
	Filter      map[string]any `json:"filter,omitempty"`
}

// QdrantClient manages HTTP communications, collection setup, and vector indexing against a Qdrant cluster.
type QdrantClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
	log     *slog.Logger
}

// NewQdrantClient creates a production-grade Qdrant client with connection pooling and timeouts.
// Returns (nil, nil) if QdrantURL is not configured.
func NewQdrantClient(cfg *config.Config) (*QdrantClient, error) {
	if cfg == nil || cfg.QdrantURL == "" {
		return nil, nil
	}

	cleanURL := strings.TrimRight(strings.TrimSpace(cfg.QdrantURL), "/")
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 25,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	qClient := &QdrantClient{
		baseURL: cleanURL,
		apiKey:  cfg.QdrantAPIKey,
		client:  client,
		log:     slog.Default().With("component", "qdrant"),
	}

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := qClient.Health(ctx); err != nil {
		qClient.log.Warn("qdrant_initial_health_check_failed", "url", utils.MaskURL(cleanURL), "error", err)
	} else {
		qClient.log.Info("qdrant_connected_successfully", "url", utils.MaskURL(cleanURL))
	}

	return qClient, nil
}

func (q *QdrantClient) doRequest(ctx context.Context, method, path string, bodyPayload any) ([]byte, int, error) {
	if q == nil || q.client == nil {
		return nil, 0, fmt.Errorf("qdrant client is uninitialized")
	}

	var reqBody io.Reader
	if bodyPayload != nil {
		data, err := json.Marshal(bodyPayload)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to marshal qdrant request: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	fullURL := fmt.Sprintf("%s%s", q.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if q.apiKey != "" {
		req.Header.Set("api-key", q.apiKey)
	}

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("qdrant request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read qdrant response: %w", err)
	}

	return respBytes, resp.StatusCode, nil
}

// Health checks the readiness of the Qdrant cluster.
func (q *QdrantClient) Health(ctx context.Context) error {
	if q == nil {
		return fmt.Errorf("qdrant client is uninitialized")
	}

	_, status, err := q.doRequest(ctx, http.MethodGet, "/readyz", nil)
	if err != nil {
		// Fallback check against root or livez
		_, status, err = q.doRequest(ctx, http.MethodGet, "/livez", nil)
	}
	if err != nil {
		return fmt.Errorf("qdrant health check failed: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("qdrant health check returned non-2xx status: %d", status)
	}
	return nil
}

// EnsureCollection verifies that the collection exists, creating it if absent.
func (q *QdrantClient) EnsureCollection(ctx context.Context, collectionName string, vectorSize int, distance string) error {
	if q == nil {
		return fmt.Errorf("qdrant client is uninitialized")
	}
	if distance == "" {
		distance = "Cosine"
	}

	// 1. Check if collection exists
	_, status, _ := q.doRequest(ctx, http.MethodGet, fmt.Sprintf("/collections/%s", collectionName), nil)
	if status == http.StatusOK {
		return nil
	}

	// 2. Create collection
	payload := map[string]any{
		"vectors": map[string]any{
			"size":     vectorSize,
			"distance": distance,
		},
	}

	resp, status, err := q.doRequest(ctx, http.MethodPut, fmt.Sprintf("/collections/%s", collectionName), payload)
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", collectionName, err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("failed to create collection %s (status %d): %s", collectionName, status, string(resp))
	}

	q.log.Info("qdrant_collection_created", "collection", collectionName, "dimension", vectorSize, "distance", distance)
	return nil
}

// UpsertPoints inserts or updates vector points with their associated metadata payloads.
func (q *QdrantClient) UpsertPoints(ctx context.Context, collectionName string, points []QdrantPoint) error {
	if q == nil {
		return fmt.Errorf("qdrant client is uninitialized")
	}
	if len(points) == 0 {
		return nil
	}

	payload := map[string]any{
		"points": points,
	}

	path := fmt.Sprintf("/collections/%s/points?wait=true", collectionName)
	resp, status, err := q.doRequest(ctx, http.MethodPut, path, payload)
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("qdrant upsert error (status %d): %s", status, string(resp))
	}
	return nil
}

// Search executes a similarity vector search and returns nearest matched items with payloads.
func (q *QdrantClient) Search(ctx context.Context, collectionName string, req QdrantSearchRequest) ([]ScoredPoint, error) {
	if q == nil {
		return nil, fmt.Errorf("qdrant client is uninitialized")
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}
	req.WithPayload = true

	path := fmt.Sprintf("/collections/%s/points/search", collectionName)
	resp, status, err := q.doRequest(ctx, http.MethodPost, path, req)
	if err != nil {
		return nil, fmt.Errorf("qdrant search failed: %w", err)
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("qdrant search error (status %d): %s", status, string(resp))
	}

	var parsed struct {
		Result []ScoredPoint `json:"result"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse search result: %w", err)
	}

	return parsed.Result, nil
}

// DeletePoints removes vectors by ID.
func (q *QdrantClient) DeletePoints(ctx context.Context, collectionName string, ids []string) error {
	if q == nil {
		return fmt.Errorf("qdrant client is uninitialized")
	}
	if len(ids) == 0 {
		return nil
	}

	payload := map[string]any{
		"points": ids,
	}

	path := fmt.Sprintf("/collections/%s/points/delete?wait=true", collectionName)
	resp, status, err := q.doRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("qdrant delete points error (status %d): %s", status, string(resp))
	}
	return nil
}
