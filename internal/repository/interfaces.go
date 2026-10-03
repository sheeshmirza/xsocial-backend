package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
)

// IUserRepository defines the persistence contract for user entities.
type IUserRepository interface {
	Create(ctx context.Context, user *models.User) error
	FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	FindByVerificationToken(ctx context.Context, token string) (*models.User, error)
	FindByResetToken(ctx context.Context, token string) (*models.User, error)
	UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) (*models.User, error)
	DeleteByID(ctx context.Context, id bson.ObjectID) error
}

// IRefreshTokenRepository defines the contract for session/refresh token lifecycle.
type IRefreshTokenRepository interface {
	Create(ctx context.Context, token *models.RefreshToken) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error)
	RevokeByID(ctx context.Context, id bson.ObjectID) error
	RevokeAllByUserID(ctx context.Context, userID bson.ObjectID) error
	DeleteExpired(ctx context.Context, now time.Time) error
}

// IBrandRepository defines workspace and team membership persistence operations.
type IBrandRepository interface {
	CreateBrand(ctx context.Context, brand *models.Brand) error
	CreateBrandWithOwner(ctx context.Context, brand *models.Brand, ownerEmail, ownerName string) error
	FindBrandByID(ctx context.Context, id bson.ObjectID) (*models.Brand, error)
	FindBrandsByUserID(ctx context.Context, userID bson.ObjectID) ([]models.Brand, error)
	UpdateBrand(ctx context.Context, id bson.ObjectID, update bson.M) (*models.Brand, error)
	DeleteBrand(ctx context.Context, id bson.ObjectID) (bool, error)
	AddMember(ctx context.Context, member *models.BrandMember) error
	FindMembersByBrandID(ctx context.Context, brandID bson.ObjectID) ([]models.BrandMember, error)
	FindMember(ctx context.Context, brandID, userID bson.ObjectID) (*models.BrandMember, error)
	RemoveMember(ctx context.Context, brandID, userID bson.ObjectID) (bool, error)
}

// ISocialHandleRepository defines connected social platform OAuth identity persistence.
type ISocialHandleRepository interface {
	Create(ctx context.Context, handle *models.SocialHandle) error
	ListByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M, skip, limit int, sort bson.D) ([]models.SocialHandle, error)
	CountByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M) (int64, error)
	FindByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.SocialHandle, error)
	UpsertByUserPlatformIdentity(ctx context.Context, userID bson.ObjectID, platform, platformID string, handle *models.SocialHandle) (*models.SocialHandle, error)
	UpdateByIDAndUserID(ctx context.Context, id, userID bson.ObjectID, update bson.M) (*models.SocialHandle, error)
	DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.SocialHandle, error)
	FindByUserPlatform(ctx context.Context, userID bson.ObjectID, platform string) ([]models.SocialHandle, error)
	FindByUserPlatforms(ctx context.Context, userID bson.ObjectID, platformList []string) ([]models.SocialHandle, error)
	FindByUserAndIDs(ctx context.Context, userID bson.ObjectID, ids []string) ([]models.SocialHandle, error)
	FindByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.SocialHandle, error)
}

// IPostRepository defines content creation, scheduling, aggregation, and querying operations.
type IPostRepository interface {
	Create(ctx context.Context, post *models.Post) error
	FindByID(ctx context.Context, id bson.ObjectID) (*models.Post, error)
	FindByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.Post, error)
	ListRecurringScheduledPosts(ctx context.Context) ([]models.Post, error)
	ListByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M, skip, limit int, sort bson.D) ([]models.Post, error)
	CountByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M) (int64, error)
	CountByFilter(ctx context.Context, filter bson.M) (int64, error)
	ListByFilter(ctx context.Context, filter bson.M, limit int) ([]models.Post, error)
	UpdateByIDAndUserID(ctx context.Context, id, userID bson.ObjectID, update bson.M) (*models.Post, error)
	UpdateByID(ctx context.Context, id bson.ObjectID, update bson.M) (*models.Post, error)
	DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.Post, error)
	DeleteManyByIDsAndUserID(ctx context.Context, ids []string, userID bson.ObjectID) (int64, error)
	FindByIDsAndUserID(ctx context.Context, ids []string, userID bson.ObjectID) ([]models.Post, error)
	AggregateDashboard(ctx context.Context, userID bson.ObjectID, filters bson.M) (map[string]int64, error)
	PlatformStats(ctx context.Context, userID bson.ObjectID, filters bson.M) ([]PlatformStatsResult, error)
	FindByApprovalToken(ctx context.Context, token string) (*models.Post, error)
}

// IPostActivityRepository defines social platform telemetry and engagement tracking operations.
type IPostActivityRepository interface {
	UpsertLinkedInActivity(ctx context.Context, userID, socialHandleID bson.ObjectID, platformPostID string, activity *models.PostActivity) (*models.PostActivity, error)
	ListByPlatformPostIDs(ctx context.Context, userID bson.ObjectID, platformPostIDs []string, platform string) ([]models.PostActivity, error)
	DeleteByPlatformPostIDs(ctx context.Context, userID bson.ObjectID, platformPostIDs []string, platform string) error
	DeleteByPostPlatformAndPlatformPostID(ctx context.Context, userID bson.ObjectID, platform, platformPostID string) error
	ListByUser(ctx context.Context, userID bson.ObjectID, filters bson.M, skip, limit int, sort bson.D) ([]models.PostActivity, error)
	CountByUser(ctx context.Context, userID bson.ObjectID, filters bson.M) (int64, error)
}

// IApiKeyRepository defines developer and API key credential verification operations.
type IApiKeyRepository interface {
	Create(ctx context.Context, key *models.ApiKey) error
	FindByKeyHash(ctx context.Context, keyHash string) (*models.ApiKey, error)
	FindByUserID(ctx context.Context, userID bson.ObjectID) ([]models.ApiKey, error)
	UpdateLastUsed(ctx context.Context, id bson.ObjectID)
	DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (bool, error)
}

// IAuditLogRepository defines enterprise compliance and security event auditing operations.
type IAuditLogRepository interface {
	Record(ctx context.Context, log *models.AuditLog)
	Find(ctx context.Context, userID *bson.ObjectID, brandID *bson.ObjectID, limit int64) ([]models.AuditLog, error)
}

// Compile-time assertions verifying concrete repository types satisfy their interfaces
var (
	_ IUserRepository         = (*UserRepository)(nil)
	_ IRefreshTokenRepository = (*RefreshTokenRepository)(nil)
	_ IBrandRepository        = (*BrandRepository)(nil)
	_ ISocialHandleRepository = (*SocialHandleRepository)(nil)
	_ IPostRepository         = (*PostRepository)(nil)
	_ IPostActivityRepository = (*PostActivityRepository)(nil)
	_ IApiKeyRepository       = (*ApiKeyRepository)(nil)
	_ IAuditLogRepository     = (*AuditLogRepository)(nil)
)
