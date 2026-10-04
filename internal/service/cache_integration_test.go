package service_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/database"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/service"
)

// Mock ApiKey Repository
type mockApiKeyRepo struct {
	findCallCount atomic.Int64
	key           *models.ApiKey
}

func (m *mockApiKeyRepo) Create(ctx context.Context, key *models.ApiKey) error {
	m.key = key
	return nil
}

func (m *mockApiKeyRepo) FindByKeyHash(ctx context.Context, keyHash string) (*models.ApiKey, error) {
	m.findCallCount.Add(1)
	if m.key != nil && m.key.KeyHash == keyHash {
		return m.key, nil
	}
	return nil, nil
}

func (m *mockApiKeyRepo) FindByUserID(ctx context.Context, userID bson.ObjectID) ([]models.ApiKey, error) {
	if m.key != nil && m.key.UserID == userID {
		return []models.ApiKey{*m.key}, nil
	}
	return nil, nil
}

func (m *mockApiKeyRepo) UpdateLastUsed(ctx context.Context, id bson.ObjectID) {}

func (m *mockApiKeyRepo) DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (bool, error) {
	if m.key != nil && m.key.ID == id && m.key.UserID == userID {
		m.key = nil
		return true, nil
	}
	return false, nil
}

func TestApiKeyService_MultiTierCacheIntegration(t *testing.T) {
	ctx := context.Background()
	userOID := bson.NewObjectID()
	keyOID := bson.NewObjectID()
	rawKey := "mc_live_1234567890abcdef12345678"
	keyHash := service.HashApiKey(rawKey)

	repo := &mockApiKeyRepo{
		key: &models.ApiKey{
			ID:          keyOID,
			UserID:      userOID,
			Name:        "Test Key",
			KeyHash:     keyHash,
			Permissions: []string{"full_access"},
		},
	}

	svc := service.NewApiKeyService(repo, nil, nil)
	mtc := database.NewMultiTierCache(nil, 1*time.Minute)
	defer mtc.Close()
	svc.SetCache(mtc)

	// 1. First validation (cache miss -> hits DB repo)
	k1, err := svc.ValidateApiKey(ctx, rawKey)
	if err != nil || k1 == nil {
		t.Fatalf("first validation failed: %v", err)
	}
	if repo.findCallCount.Load() != 1 {
		t.Fatalf("expected 1 repo call on cache miss, got %d", repo.findCallCount.Load())
	}

	// 2. Second validation (cache hit -> sub-microsecond in-memory retrieval, repo NOT called)
	k2, err := svc.ValidateApiKey(ctx, rawKey)
	if err != nil || k2 == nil {
		t.Fatalf("second validation failed: %v", err)
	}
	if repo.findCallCount.Load() != 1 {
		t.Fatalf("expected repo call count to remain 1 on cache hit, got %d", repo.findCallCount.Load())
	}

	// 3. Revoke key (must invalidate multi-tier cache)
	err = svc.RevokeApiKey(ctx, keyOID, userOID)
	if err != nil {
		t.Fatalf("revocation failed: %v", err)
	}

	// 4. Third validation after revocation (cache evicted -> repo returns nil -> unauthorized)
	_, err = svc.ValidateApiKey(ctx, rawKey)
	if err == nil {
		t.Fatal("expected unauthorized error after revocation and cache eviction")
	}
	if repo.findCallCount.Load() != 2 {
		t.Fatalf("expected 2 repo calls after cache eviction, got %d", repo.findCallCount.Load())
	}
}

// Mock User Repository
type mockUserRepo struct {
	findCallCount atomic.Int64
	user          *models.User
}

func (m *mockUserRepo) FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	m.findCallCount.Add(1)
	if m.user != nil && m.user.ID == id {
		return m.user, nil
	}
	return nil, nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.user != nil && m.user.Email == email {
		return m.user, nil
	}
	return nil, nil
}

func (m *mockUserRepo) FindByVerificationToken(ctx context.Context, token string) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRepo) FindByResetToken(ctx context.Context, token string) (*models.User, error) {
	return nil, nil
}

func (m *mockUserRepo) Create(ctx context.Context, u *models.User) error {
	m.user = u
	return nil
}

func (m *mockUserRepo) UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) (*models.User, error) {
	if m.user != nil && m.user.ID == id {
		if fn, ok := update["fullName"].(string); ok {
			m.user.FullName = fn
		}
		return m.user, nil
	}
	return nil, nil
}

func (m *mockUserRepo) DeleteByID(ctx context.Context, id bson.ObjectID) error {
	m.user = nil
	return nil
}

func TestUserService_MultiTierCacheIntegration(t *testing.T) {
	ctx := context.Background()
	userOID := bson.NewObjectID()
	user := &models.User{
		ID:       userOID,
		Email:    "test@example.com",
		FullName: "Original Name",
	}

	repo := &mockUserRepo{user: user}
	svc := service.NewUserService(nil, repo, nil, nil)
	mtc := database.NewMultiTierCache(nil, 1*time.Minute)
	defer mtc.Close()
	svc.SetCache(mtc)

	// 1. Initial retrieval: cache miss -> queries repo
	u1, err := svc.GetUserByID(ctx, userOID)
	if err != nil || u1 == nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if repo.findCallCount.Load() != 1 {
		t.Fatalf("expected 1 repo call, got %d", repo.findCallCount.Load())
	}

	// 2. Second retrieval: cache hit -> repo not called
	u2, err := svc.GetUserByID(ctx, userOID)
	if err != nil || u2 == nil {
		t.Fatalf("failed to get user on cache hit: %v", err)
	}
	if repo.findCallCount.Load() != 1 {
		t.Fatalf("expected repo call count to remain 1, got %d", repo.findCallCount.Load())
	}

	// 3. Update profile: must invalidate cache
	_, err = svc.UpdateProfile(ctx, userOID, "Updated Name", "")
	if err != nil {
		t.Fatalf("failed to update profile: %v", err)
	}

	// 4. Third retrieval: cache miss -> queries repo and reflects updated name
	u3, err := svc.GetUserByID(ctx, userOID)
	if err != nil || u3 == nil {
		t.Fatalf("failed to get user after update: %v", err)
	}
	if repo.findCallCount.Load() != 2 {
		t.Fatalf("expected 2 repo calls after update invalidation, got %d", repo.findCallCount.Load())
	}
	if u3.FullName != "Updated Name" {
		t.Fatalf("expected updated name %q, got %q", "Updated Name", u3.FullName)
	}
}

// Mock Post Repository
type mockPostRepo struct {
	findCallCount atomic.Int64
	post          *models.Post
}

func (m *mockPostRepo) Create(ctx context.Context, p *models.Post) error {
	m.post = p
	return nil
}

func (m *mockPostRepo) FindByID(ctx context.Context, id bson.ObjectID) (*models.Post, error) {
	if m.post != nil && m.post.ID == id {
		return m.post, nil
	}
	return nil, nil
}

func (m *mockPostRepo) FindByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.Post, error) {
	if m.post != nil && m.post.ID == id && m.post.UserID == userID {
		return m.post, nil
	}
	return nil, nil
}

func (m *mockPostRepo) ListRecurringScheduledPosts(ctx context.Context) ([]models.Post, error) {
	return nil, nil
}

func (m *mockPostRepo) FindByApprovalToken(ctx context.Context, token string) (*models.Post, error) {
	m.findCallCount.Add(1)
	if m.post != nil && m.post.ApprovalToken == token {
		return m.post, nil
	}
	return nil, nil
}

func (m *mockPostRepo) FindByIDsAndUserID(ctx context.Context, ids []string, userID bson.ObjectID) ([]models.Post, error) {
	return nil, nil
}

func (m *mockPostRepo) ListByUserID(ctx context.Context, userID bson.ObjectID, filter bson.M, skip, limit int, sort bson.D) ([]models.Post, error) {
	return nil, nil
}

func (m *mockPostRepo) CountByUserID(ctx context.Context, userID bson.ObjectID, filter bson.M) (int64, error) {
	return 0, nil
}

func (m *mockPostRepo) CountByFilter(ctx context.Context, filter bson.M) (int64, error) {
	return 0, nil
}

func (m *mockPostRepo) ListByFilter(ctx context.Context, filter bson.M, limit int) ([]models.Post, error) {
	return nil, nil
}

func (m *mockPostRepo) UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) (*models.Post, error) {
	if m.post != nil && m.post.ID == id {
		if st, ok := update["reviewStatus"].(string); ok {
			m.post.ReviewStatus = st
		}
		return m.post, nil
	}
	return nil, nil
}

func (m *mockPostRepo) UpdateByIDAndUserID(ctx context.Context, id, userID bson.ObjectID, update bson.M) (*models.Post, error) {
	return m.UpdateByID(ctx, id, update)
}

func (m *mockPostRepo) DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.Post, error) {
	p := m.post
	m.post = nil
	return p, nil
}

func (m *mockPostRepo) DeleteManyByIDsAndUserID(ctx context.Context, ids []string, userID bson.ObjectID) (int64, error) {
	return 0, nil
}

func (m *mockPostRepo) AggregateDashboard(ctx context.Context, userID bson.ObjectID, filters bson.M) (map[string]int64, error) {
	return nil, nil
}

func (m *mockPostRepo) PlatformStats(ctx context.Context, userID bson.ObjectID, filter bson.M) ([]repository.PlatformStatsResult, error) {
	return nil, nil
}

func TestPostService_MultiTierCacheApprovalToken(t *testing.T) {
	ctx := context.Background()
	postOID := bson.NewObjectID()
	token := "approval-token-xyz-123"

	repo := &mockPostRepo{
		post: &models.Post{
			ID:            postOID,
			ReviewStatus:  "pending",
			ApprovalToken: token,
		},
	}

	svc := service.NewPostService(repo, nil, nil, nil, nil, nil, nil, nil, nil)
	mtc := database.NewMultiTierCache(nil, 1*time.Minute)
	defer mtc.Close()
	svc.SetCache(mtc)

	// 1. First retrieval: calls repo
	p1, err := svc.FindByApprovalToken(ctx, token)
	if err != nil || p1 == nil {
		t.Fatalf("first find failed: %v", err)
	}
	if repo.findCallCount.Load() != 1 {
		t.Fatalf("expected 1 repo call, got %d", repo.findCallCount.Load())
	}

	// 2. Second retrieval: hits cache, repo not called
	p2, err := svc.FindByApprovalToken(ctx, token)
	if err != nil || p2 == nil {
		t.Fatalf("second find failed: %v", err)
	}
	if repo.findCallCount.Load() != 1 {
		t.Fatalf("expected repo call count to remain 1, got %d", repo.findCallCount.Load())
	}

	// 3. Action taken on approval token: invalidates cache
	_, err = svc.PublicReviewAction(ctx, token, "approve", "Reviewer Jane", "Approved!")
	if err != nil {
		t.Fatalf("review action failed: %v", err)
	}

	// 4. Third retrieval: cache was invalidated, calls repo again
	p3, err := svc.FindByApprovalToken(ctx, token)
	if err != nil || p3 == nil {
		t.Fatalf("third find failed: %v", err)
	}
	if repo.findCallCount.Load() != 3 { // Note: PublicReviewAction looks up post before updating (call 2), then FindByApprovalToken is call 3
		t.Fatalf("expected 3 repo calls after action and cache invalidation, got %d", repo.findCallCount.Load())
	}
	if p3.ReviewStatus != "approved" {
		t.Fatalf("expected status 'approved', got %q", p3.ReviewStatus)
	}
}
