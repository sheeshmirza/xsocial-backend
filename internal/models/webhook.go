package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Webhook struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    bson.ObjectID `bson:"userId" json:"userId"`
	BrandID   *bson.ObjectID `bson:"brandId,omitempty" json:"brandId,omitempty"`
	URL       string        `bson:"url" json:"url"`
	Events    []string      `bson:"events" json:"events"`
	Secret    string        `bson:"secret" json:"-"`
	Status    string        `bson:"status" json:"status"` // active, disabled
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type CreateWebhookRequest struct {
	URL     string   `json:"url" binding:"required,url"`
	Events  []string `json:"events" binding:"required"`
	BrandID string   `json:"brandId"`
}

