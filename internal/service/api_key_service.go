package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/database"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type ApiKeyService struct {
	repo  *repository.ApiKeyRepository
	redis *database.RedisClient
}

func NewApiKeyService(repo *repository.ApiKeyRepository, redis *database.RedisClient) *ApiKeyService {
	return &ApiKeyService{
		repo:  repo,
		redis: redis,
	}
}

func HashApiKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func generateSecureToken(prefix string, byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

func (s *ApiKeyService) CreateApiKey(ctx context.Context, userID bson.ObjectID, req models.CreateApiKeyRequest) (*models.CreateApiKeyResponse, error) {
	rawKey, err := generateSecureToken("mc_live_", 24)
	if err != nil {
		return nil, utils.InternalServerError("Failed to generate API key")
	}

	keyHash := HashApiKey(rawKey)
	lastFour := rawKey[len(rawKey)-4:]

	perms := req.Permissions
	if len(perms) == 0 {
		perms = []string{"full_access"}
	}

	var brandID *bson.ObjectID
	if req.BrandID != "" {
		if bid, err := bson.ObjectIDFromHex(req.BrandID); err == nil {
			brandID = &bid
		}
	}

	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		exp := time.Now().AddDate(0, 0, req.ExpiresInDays)
		expiresAt = &exp
	}

	apiKey := &models.ApiKey{
		UserID:      userID,
		BrandID:     brandID,
		Name:        req.Name,
		KeyHash:     keyHash,
		Prefix:      "mc_live_",
		LastFour:    lastFour,
		Permissions: perms,
		ExpiresAt:   expiresAt,
	}

	if err := s.repo.Create(ctx, apiKey); err != nil {
		return nil, utils.InternalServerError("Failed to store API key")
	}

	// Cache in Redis for fast authentication (5 min TTL)
	s.cacheKeyInRedis(ctx, keyHash, apiKey)

	brandIDStr := ""
	if apiKey.BrandID != nil {
		brandIDStr = apiKey.BrandID.Hex()
	}

	view := models.ApiKeyView{
		ID:          apiKey.ID.Hex(),
		Name:        apiKey.Name,
		Prefix:      apiKey.Prefix,
		LastFour:    apiKey.LastFour,
		Permissions: apiKey.Permissions,
		BrandID:     brandIDStr,
		LastUsedAt:  apiKey.LastUsedAt,
		ExpiresAt:   apiKey.ExpiresAt,
		CreatedAt:   apiKey.CreatedAt,
	}

	return &models.CreateApiKeyResponse{
		ApiKey: view,
		Key:    rawKey,
	}, nil
}

func (s *ApiKeyService) ValidateApiKey(ctx context.Context, rawKey string) (*models.ApiKey, error) {
	keyHash := HashApiKey(rawKey)

	// Check Redis cache first
	if s.redis != nil {
		if cachedJSON, err := s.redis.Get(ctx, "apikey:"+keyHash); err == nil && cachedJSON != "" {
			var cached models.ApiKey
			if json.Unmarshal([]byte(cachedJSON), &cached) == nil {
				go s.repo.UpdateLastUsed(context.Background(), cached.ID)
				return &cached, nil
			}
		}
	}

	key, err := s.repo.FindByKeyHash(ctx, keyHash)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, utils.Unauthorized("Invalid API key")
	}

	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		return nil, utils.Unauthorized("API key has expired")
	}

	// Cache key in Redis
	s.cacheKeyInRedis(ctx, keyHash, key)

	// Update last used asynchronously
	go s.repo.UpdateLastUsed(context.Background(), key.ID)

	return key, nil
}

func (s *ApiKeyService) cacheKeyInRedis(ctx context.Context, keyHash string, key *models.ApiKey) {
	if s.redis == nil {
		return
	}
	if b, err := json.Marshal(key); err == nil {
		_ = s.redis.Set(ctx, "apikey:"+keyHash, string(b), 5*time.Minute)
	}
}

func (s *ApiKeyService) ListApiKeys(ctx context.Context, userID bson.ObjectID) ([]models.ApiKeyView, error) {
	keys, err := s.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, utils.InternalServerError("Failed to retrieve API keys")
	}

	views := make([]models.ApiKeyView, len(keys))
	for i, k := range keys {
		brandIDStr := ""
		if k.BrandID != nil {
			brandIDStr = k.BrandID.Hex()
		}
		views[i] = models.ApiKeyView{
			ID:          k.ID.Hex(),
			Name:        k.Name,
			Prefix:      k.Prefix,
			LastFour:    k.LastFour,
			Permissions: k.Permissions,
			BrandID:     brandIDStr,
			LastUsedAt:  k.LastUsedAt,
			ExpiresAt:   k.ExpiresAt,
			CreatedAt:   k.CreatedAt,
		}
	}
	return views, nil
}

func (s *ApiKeyService) RevokeApiKey(ctx context.Context, id, userID bson.ObjectID) error {
	deleted, err := s.repo.DeleteByIDAndUserID(ctx, id, userID)
	if err != nil {
		return utils.InternalServerError("Failed to revoke API key")
	}
	if !deleted {
		return utils.NotFound("API key not found")
	}
	return nil
}

func HasPermission(key *models.ApiKey, requiredPerm string) bool {
	if key == nil {
		return false
	}
	for _, p := range key.Permissions {
		if strings.EqualFold(p, "full_access") || strings.EqualFold(p, requiredPerm) {
			return true
		}
	}
	return false
}
