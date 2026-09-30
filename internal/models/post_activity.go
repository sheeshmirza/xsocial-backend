package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type MediaSnapshot struct {
	Type     string `bson:"type" json:"type"`
	URL      string `bson:"url" json:"url"`
	MimeType string `bson:"mimeType" json:"mimeType"`
}

type ContentSnapshot struct {
	Title              string          `bson:"title" json:"title"`
	Caption            string          `bson:"caption" json:"caption"`
	Hashtags           []string        `bson:"hashtags" json:"hashtags"`
	Mentions           []string        `bson:"mentions" json:"mentions"`
	Media              []MediaSnapshot `bson:"media" json:"media"`
	PublishedTime      *time.Time      `bson:"publishedTime" json:"publishedTime"`
	PublishedExpiresAt *time.Time      `bson:"publishedExpiresAt" json:"publishedExpiresAt"`
}

type PostActivity struct {
	ID              bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	UserID          bson.ObjectID   `bson:"userId" json:"userId"`
	SocialHandleID  bson.ObjectID   `bson:"socialHandleId" json:"socialHandleId"`
	Platform        string          `bson:"platform" json:"platform"` // "linkedin"
	PlatformPostID  string          `bson:"platformPostId" json:"platformPostId"`
	PlatformPostURN string          `bson:"platformPostUrn" json:"platformPostUrn"`
	PlatformPostURL string          `bson:"platformPostUrl" json:"platformPostUrl"`
	ContentSnapshot ContentSnapshot `bson:"contentSnapshot" json:"contentSnapshot"`
	CreatedAt       time.Time       `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time       `bson:"updatedAt" json:"updatedAt"`
}
