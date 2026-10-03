package models

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ApiKey struct {
	ID          bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID      bson.ObjectID  `bson:"userId" json:"userId"`
	BrandID     *bson.ObjectID `bson:"brandId,omitempty" json:"brandId,omitempty"`
	Name        string         `bson:"name" json:"name"`
	KeyHash     string         `bson:"keyHash" json:"-"`
	Prefix      string         `bson:"prefix" json:"prefix"`
	LastFour    string         `bson:"lastFour" json:"lastFour"`
	Permissions []string       `bson:"permissions" json:"permissions"`
	LastUsedAt  *time.Time     `bson:"lastUsedAt,omitempty" json:"lastUsedAt,omitempty"`
	ExpiresAt   *time.Time     `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"`
	CreatedAt   time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time      `bson:"updatedAt" json:"updatedAt"`
}

type ApiKeyView struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	LastFour    string     `json:"lastFour"`
	Permissions []string   `json:"permissions"`
	BrandID     string     `json:"brandId,omitempty"`
	LastUsedAt  *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt   *time.Time `json:"expiresAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type CreateApiKeyRequest struct {
	Name          string   `json:"name" binding:"required"`
	Permissions   []string `json:"permissions"`
	BrandID       string   `json:"brandId"`
	ExpiresInDays int      `json:"expiresInDays"`
}

func (r *CreateApiKeyRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return fmt.Errorf("API key name is required")
	}
	if len(r.Name) > 100 {
		return fmt.Errorf("API key name must not exceed 100 characters")
	}
	if r.ExpiresInDays < 0 || r.ExpiresInDays > 365 {
		return fmt.Errorf("expiresInDays must be between 1 and 365 days")
	}
	return nil
}

type CreateApiKeyResponse struct {
	ApiKey ApiKeyView `json:"apiKey"`
	Key    string     `json:"key"` // Plaintext key only shown once upon creation
}
