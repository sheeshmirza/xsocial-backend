package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type AuditLog struct {
	ID           bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID       *bson.ObjectID `bson:"userId,omitempty" json:"userId,omitempty"`
	BrandID      *bson.ObjectID `bson:"brandId,omitempty" json:"brandId,omitempty"`
	Actor        string         `bson:"actor" json:"actor"`
	Action       string         `bson:"action" json:"action"`
	ResourceType string         `bson:"resourceType" json:"resourceType"`
	ResourceID   string         `bson:"resourceId" json:"resourceId"`
	IPAddress    string         `bson:"ipAddress,omitempty" json:"ipAddress,omitempty"`
	UserAgent    string         `bson:"userAgent,omitempty" json:"userAgent,omitempty"`
	CreatedAt    time.Time      `bson:"createdAt" json:"createdAt"`
}
