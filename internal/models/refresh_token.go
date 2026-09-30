package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RefreshToken struct {
	ID             bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID         bson.ObjectID  `bson:"userId" json:"userId"`
	TokenHash      string         `bson:"tokenHash" json:"tokenHash"`
	TokenExpiresAt time.Time      `bson:"tokenExpiresAt" json:"tokenExpiresAt"`
	TokenRevokedAt *time.Time     `bson:"tokenRevokedAt" json:"tokenRevokedAt"`
	UserAgent      string         `bson:"userAgent" json:"userAgent"`
	UserIPAddress  string         `bson:"userIPAddress" json:"userIPAddress"`
	CreatedAt      time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time      `bson:"updatedAt" json:"updatedAt"`
}
