package migrations

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (m *Migrator) registerBuiltInMigrations() {
	m.Register(&Migration001InitialSchema{})
	m.Register(&Migration002PostCollaboration{})
	m.Register(&Migration003SocialPlatformUniqueness{})
	m.Register(&Migration004AuditAndSecurityTTL{})
}

func createIndexesSafe(ctx context.Context, coll *mongo.Collection, models []mongo.IndexModel) error {
	for _, model := range models {
		_, err := coll.Indexes().CreateOne(ctx, model)
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "IndexOptionsConflict") ||
				strings.Contains(errStr, "IndexKeySpecsConflict") ||
				strings.Contains(errStr, "already exists") {
				continue
			}
			return err
		}
	}
	return nil
}

// Migration 001: Initial Core Schema & Indexes
type Migration001InitialSchema struct{}

func (m *Migration001InitialSchema) Version() int  { return 1 }
func (m *Migration001InitialSchema) Name() string { return "001_initial_schema_indexes" }

func (m *Migration001InitialSchema) Up(ctx context.Context, db *mongo.Database) error {
	// 1. Users collection
	users := db.Collection("users")
	if err := createIndexesSafe(ctx, users, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_user_email"),
		},
		{
			Keys:    bson.D{{Key: "emailVerificationToken", Value: 1}},
			Options: options.Index().SetSparse(true).SetName("idx_user_email_verif_token"),
		},
		{
			Keys:    bson.D{{Key: "resetPasswordToken", Value: 1}},
			Options: options.Index().SetSparse(true).SetName("idx_user_reset_pwd_token"),
		},
	}); err != nil {
		return err
	}

	// 2. Brands and Brand Members
	brands := db.Collection("brands")
	if err := createIndexesSafe(ctx, brands, []mongo.IndexModel{
		{Keys: bson.D{{Key: "ownerId", Value: 1}}},
	}); err != nil {
		return err
	}

	members := db.Collection("brand_members")
	if err := createIndexesSafe(ctx, members, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "brandId", Value: 1}, {Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_brand_member"),
		},
		{Keys: bson.D{{Key: "userId", Value: 1}}},
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
	}); err != nil {
		return err
	}

	// 3. API Keys
	apiKeys := db.Collection("api_keys")
	return createIndexesSafe(ctx, apiKeys, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "keyHash", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_api_key_hash"),
		},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "brandId", Value: 1}}, Options: options.Index().SetSparse(true)},
	})
}

func (m *Migration001InitialSchema) Down(ctx context.Context, db *mongo.Database) error {
	_ = db.Collection("users").Indexes().DropOne(ctx, "uniq_user_email")
	_ = db.Collection("brand_members").Indexes().DropOne(ctx, "uniq_brand_member")
	_ = db.Collection("api_keys").Indexes().DropOne(ctx, "uniq_api_key_hash")
	return nil
}

// Migration 002: Post Collaboration & Approval Indexes
type Migration002PostCollaboration struct{}

func (m *Migration002PostCollaboration) Version() int  { return 2 }
func (m *Migration002PostCollaboration) Name() string { return "002_post_collaboration_indexes" }

func (m *Migration002PostCollaboration) Up(ctx context.Context, db *mongo.Database) error {
	posts := db.Collection("posts")
	return createIndexesSafe(ctx, posts, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "scheduledTime", Value: 1}, {Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "status", Value: 1}}},
		{
			Keys:    bson.D{{Key: "approvalToken", Value: 1}},
			Options: options.Index().SetSparse(true).SetName("idx_post_approval_token"),
		},
		{Keys: bson.D{{Key: "publishedExpiresAt", Value: 1}}},
	})
}

func (m *Migration002PostCollaboration) Down(ctx context.Context, db *mongo.Database) error {
	_ = db.Collection("posts").Indexes().DropOne(ctx, "idx_post_approval_token")
	return nil
}

// Migration 003: Social Platform Unique Identity & Token Expiry
type Migration003SocialPlatformUniqueness struct{}

func (m *Migration003SocialPlatformUniqueness) Version() int  { return 3 }
func (m *Migration003SocialPlatformUniqueness) Name() string { return "003_social_platform_uniqueness" }

func (m *Migration003SocialPlatformUniqueness) Up(ctx context.Context, db *mongo.Database) error {
	handles := db.Collection("socialhandles")
	_ = handles.Indexes().DropOne(ctx, "uniq_user_platform")

	return createIndexesSafe(ctx, handles, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "platform", Value: 1},
				{Key: "platformId", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_user_platform_identity"),
		},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "brandId", Value: 1}}},
		{Keys: bson.D{{Key: "platformAccessTokenExpiry", Value: 1}}},
	})
}

func (m *Migration003SocialPlatformUniqueness) Down(ctx context.Context, db *mongo.Database) error {
	_ = db.Collection("socialhandles").Indexes().DropOne(ctx, "uniq_user_platform_identity")
	return nil
}

// Migration 004: Audit Log Indexes & Auto-Expiring Refresh Token TTL
type Migration004AuditAndSecurityTTL struct{}

func (m *Migration004AuditAndSecurityTTL) Version() int  { return 4 }
func (m *Migration004AuditAndSecurityTTL) Name() string { return "004_audit_and_security_ttl" }

func (m *Migration004AuditAndSecurityTTL) Up(ctx context.Context, db *mongo.Database) error {
	// Refresh tokens TTL
	rf := db.Collection("refreshtokens")
	if err := createIndexesSafe(ctx, rf, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tokenHash", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("uniq_rf_token_hash"),
		},
		{Keys: bson.D{{Key: "userId", Value: 1}}},
		{
			Keys:    bson.D{{Key: "tokenExpiresAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0).SetName("ttl_rf_token_expires"),
		},
	}); err != nil {
		return err
	}

	// Audit logs
	audit := db.Collection("audit_logs")
	return createIndexesSafe(ctx, audit, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "resourceType", Value: 1}, {Key: "resourceId", Value: 1}}},
	})
}

func (m *Migration004AuditAndSecurityTTL) Down(ctx context.Context, db *mongo.Database) error {
	_ = db.Collection("refreshtokens").Indexes().DropOne(ctx, "ttl_rf_token_expires")
	return nil
}
