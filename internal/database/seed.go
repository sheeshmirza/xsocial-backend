package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"

	"xsocial-backend/internal/models"
)

type SeedSummary struct {
	UsersCreated   int `json:"usersCreated"`
	BrandsCreated  int `json:"brandsCreated"`
	PostsCreated   int `json:"postsCreated"`
	HandlesCreated int `json:"handlesCreated"`
	LogsCreated    int `json:"logsCreated"`
}

// SeedInitialData populates the database with realistic demo accounts,
// brands, sample posts, and workspace structures. It is strictly idempotent.
func SeedInitialData(ctx context.Context, db *mongo.Database) (*SeedSummary, error) {
	summary := &SeedSummary{}

	usersColl := db.Collection("users")
	brandsColl := db.Collection("brands")
	membersColl := db.Collection("brand_members")
	postsColl := db.Collection("posts")
	handlesColl := db.Collection("socialhandles")
	auditColl := db.Collection("audit_logs")

	// 1. Seed Default Admin User
	adminEmail := "admin@xsocial.io"
	var existingAdmin models.User
	err := usersColl.FindOne(ctx, bson.M{"email": adminEmail}).Decode(&existingAdmin)

	var adminID bson.ObjectID
	if err == mongo.ErrNoDocuments {
		pwdHash, _ := bcrypt.GenerateFromPassword([]byte("AdminPassword123!"), 12)
		now := time.Now()
		adminUser := models.User{
			Email:      adminEmail,
			Password:   string(pwdHash),
			FullName:   "System Administrator",
			IsVerified: true,
			Timezone:   "America/New_York",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		res, err := usersColl.InsertOne(ctx, adminUser)
		if err != nil {
			return nil, fmt.Errorf("failed to seed admin user: %w", err)
		}
		adminID = res.InsertedID.(bson.ObjectID)
		summary.UsersCreated++
		log.Printf("[Seeder] Created default admin user (%s)", adminEmail)
	} else if err == nil {
		adminID = existingAdmin.ID
	} else {
		return nil, err
	}

	// 2. Seed Demo Brand / Workspace
	brandName := "Acme Global Media"
	var existingBrand models.Brand
	err = brandsColl.FindOne(ctx, bson.M{"name": brandName, "ownerId": adminID}).Decode(&existingBrand)

	var brandID bson.ObjectID
	if err == mongo.ErrNoDocuments {
		now := time.Now()
		brand := models.Brand{
			OwnerID:     adminID,
			Name:        brandName,
			Description: "Enterprise Social Media Management & Distribution Workspace",
			LogoURL:     "https://images.unsplash.com/photo-1618005182384-a83a8bd57fbe?w=200",
			Timezone:    "America/New_York",
			Currency:    "USD",
			WebsiteURL:  "https://acme.example.com",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		bRes, err := brandsColl.InsertOne(ctx, brand)
		if err != nil {
			return nil, fmt.Errorf("failed to seed brand: %w", err)
		}
		brandID = bRes.InsertedID.(bson.ObjectID)
		summary.BrandsCreated++

		// Add owner member
		ownerMember := models.BrandMember{
			BrandID:   brandID,
			UserID:    adminID,
			Email:     adminEmail,
			Name:      "System Administrator",
			Role:      models.BrandRoleOwner,
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, _ = membersColl.InsertOne(ctx, ownerMember)
		log.Printf("[Seeder] Created demo brand workspace (%s)", brandName)
	} else if err == nil {
		brandID = existingBrand.ID
	} else {
		return nil, err
	}

	// 3. Seed Mock Social Handles
	var existingHandle models.SocialHandle
	err = handlesColl.FindOne(ctx, bson.M{"userId": adminID, "platform": models.PlatformInstagram}).Decode(&existingHandle)
	if err == mongo.ErrNoDocuments {
		now := time.Now()
		expiry := now.Add(60 * 24 * time.Hour)
		handle := models.SocialHandle{
			UserID:                     adminID,
			BrandID:                    &brandID,
			Platform:                   models.PlatformInstagram,
			PlatformID:                 "17841400000000001",
			PlatformUsername:           "acme_global",
			PlatformDisplayName:        "Acme Global Media Official",
			PlatformAccessToken:        "mock_ig_access_token_demo",
			PlatformRefreshToken:       "mock_ig_refresh_token_demo",
			PlatformAccessTokenExpiry:  &expiry,
			PlatformRefreshTokenExpiry: &expiry,
			PlatformMetadata: map[string]any{
				"accountType": "business",
				"followers":   125000,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		_, err = handlesColl.InsertOne(ctx, handle)
		if err == nil {
			summary.HandlesCreated++
			log.Println("[Seeder] Created demo Instagram social handle")
		}
	}

	// 4. Seed Sample Posts (Draft & Scheduled)
	var existingPost models.Post
	err = postsColl.FindOne(ctx, bson.M{"userId": adminID, "title": "Welcome to xSocial Platform"}).Decode(&existingPost)
	if err == mongo.ErrNoDocuments {
		now := time.Now()
		scheduledTime := now.Add(24 * time.Hour)
		samplePost := models.Post{
			UserID:            adminID,
			BrandID:           &brandID,
			Title:             "Welcome to xSocial Platform",
			Caption:           "Supercharge your brand's global engagement with seamless multi-platform publishing and AI-assisted workflows! 🚀✨\n\n#SocialMedia #Growth #Enterprise",
			Hashtags:          []string{"SocialMedia", "Growth", "Enterprise"},
			PostType:          "mixed",
			Visibility:        "public",
			Timezone:          "America/New_York",
			ScheduledTime:     &scheduledTime,
			Recurrence:        models.RecurrenceOnce,
			Status:            models.PostStatusScheduled,
			SelectedPlatforms: []string{"instagram", "linkedin"},
			Media: []models.MediaItem{
				{
					Type:     "image",
					URL:      "https://images.unsplash.com/photo-1460925895917-afdab827c52f?w=1200",
					MimeType: "image/jpeg",
				},
			},
			ReviewStatus: "approved",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		_, err = postsColl.InsertOne(ctx, samplePost)
		if err == nil {
			summary.PostsCreated++
			log.Println("[Seeder] Created demo scheduled post")
		}
	}

	// 5. Seed Initial Audit Log
	auditEntry := models.AuditLog{
		UserID:       &adminID,
		BrandID:      &brandID,
		Actor:        adminEmail,
		Action:       "system.seed",
		ResourceType: "workspace",
		ResourceID:   brandID.Hex(),
		IPAddress:    "127.0.0.1",
		UserAgent:    "xSocial-Database-Seeder/1.0",
		CreatedAt:    time.Now(),
	}
	_, _ = auditColl.InsertOne(ctx, auditEntry)
	summary.LogsCreated++

	log.Printf("[Seeder] Database seeding completed: %+v", summary)
	return summary, nil
}
