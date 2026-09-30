package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type CompetitorSnapshot struct {
	Date           string  `bson:"date" json:"date"` // YYYY-MM-DD
	Followers      int64   `bson:"followers" json:"followers"`
	PostsCount     int64   `bson:"postsCount" json:"postsCount"`
	EngagementRate float64 `bson:"engagementRate" json:"engagementRate"`
}

type Competitor struct {
	ID             bson.ObjectID        `bson:"_id,omitempty" json:"id"`
	BrandID        bson.ObjectID        `bson:"brandId" json:"brandId"`
	Platform       string               `bson:"platform" json:"platform"` // instagram, facebook, youtube, twitter, twitch
	Handle         string               `bson:"handle" json:"handle"`
	Name           string               `bson:"name" json:"name"`
	AvatarURL      string               `bson:"avatarUrl" json:"avatarUrl"`
	ExternalID     string               `bson:"externalId" json:"externalId"`
	Followers      int64                `bson:"followers" json:"followers"`
	Following      int64                `bson:"following" json:"following"`
	PostsCount     int64                `bson:"postsCount" json:"postsCount"`
	EngagementRate float64              `bson:"engagementRate" json:"engagementRate"`
	History        []CompetitorSnapshot `bson:"history" json:"history"`
	LastSyncedAt   time.Time            `bson:"lastSyncedAt" json:"lastSyncedAt"`
	CreatedAt      time.Time            `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time            `bson:"updatedAt" json:"updatedAt"`
}

type AddCompetitorRequest struct {
	Platform string `json:"platform" binding:"required"`
	Handle   string `json:"handle" binding:"required"`
}
