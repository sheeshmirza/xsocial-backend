package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type SocialHandle struct {
	ID                         bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID                     bson.ObjectID  `bson:"userId" json:"userId"`
	BrandID                    *bson.ObjectID `bson:"brandId,omitempty" json:"brandId,omitempty"`
	Platform                   string         `bson:"platform" json:"platform"` // INSTAGRAM or LINKEDIN
	PlatformID                 string         `bson:"platformId" json:"platformId"`
	PlatformUsername           string         `bson:"platformUsername" json:"platformUsername"`
	PlatformDisplayName        string         `bson:"platformDisplayName" json:"platformDisplayName"`
	PlatformAccessToken        string         `bson:"platformAccessToken" json:"platformAccessToken"`
	PlatformRefreshToken       string         `bson:"platformRefreshToken" json:"platformRefreshToken"`
	PlatformAccessTokenExpiry  *time.Time     `bson:"platformAccessTokenExpiry" json:"platformAccessTokenExpiry"`
	PlatformRefreshTokenExpiry *time.Time     `bson:"platformRefreshTokenExpiry" json:"platformRefreshTokenExpiry"`
	PlatformMetadata           map[string]any `bson:"platformMetadata" json:"platformMetadata"`
	CreatedAt                  time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt                  time.Time      `bson:"updatedAt" json:"updatedAt"`

	// Non-persisted metadata for responses
	LinkedPageCount int    `bson:"-" json:"linkedPageCount,omitempty"`
	Warning         string `bson:"-" json:"warning,omitempty"`
}
