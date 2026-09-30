package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type AutolistSlot struct {
	DayOfWeek int `bson:"dayOfWeek" json:"dayOfWeek"` // 0 (Sunday) to 6 (Saturday)
	Hour      int `bson:"hour" json:"hour"`           // 0 to 23
	Minute    int `bson:"minute" json:"minute"`       // 0 to 59
}

type AutolistPost struct {
	ID               string      `bson:"id" json:"id"`
	Content          string      `bson:"content" json:"content"`
	Media            []MediaItem `bson:"media" json:"media"`
	Status           string      `bson:"status" json:"status"` // active, paused, completed
	RepetitionsCount int         `bson:"repetitionsCount" json:"repetitionsCount"`
	MaxRepetitions   int         `bson:"maxRepetitions" json:"maxRepetitions"` // 0 for infinite
	Order            int         `bson:"order" json:"order"`
	LastPublishedAt  *time.Time  `bson:"lastPublishedAt,omitempty" json:"lastPublishedAt,omitempty"`
}

type Autolist struct {
	ID              bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	BrandID         bson.ObjectID  `bson:"brandId" json:"brandId"`
	Name            string         `bson:"name" json:"name"`
	Description     string         `bson:"description" json:"description"`
	TargetPlatforms []string       `bson:"targetPlatforms" json:"targetPlatforms"`
	TimeSlots       []AutolistSlot `bson:"timeSlots" json:"timeSlots"`
	Posts           []AutolistPost `bson:"posts" json:"posts"`
	IsActive        bool           `bson:"isActive" json:"isActive"`
	IsCircular      bool           `bson:"isCircular" json:"isCircular"` // Repeat evergreen content
	CreatedAt       time.Time      `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time      `bson:"updatedAt" json:"updatedAt"`
}

type RSSFeed struct {
	ID                   bson.ObjectID `bson:"_id,omitempty" json:"id"`
	BrandID              bson.ObjectID `bson:"brandId" json:"brandId"`
	FeedURL              string        `bson:"feedUrl" json:"feedUrl"`
	TargetPlatforms      []string      `bson:"targetPlatforms" json:"targetPlatforms"`
	Template             string        `bson:"template" json:"template"` // e.g. "{title} - {link} #news"
	CheckIntervalMinutes int           `bson:"checkIntervalMinutes" json:"checkIntervalMinutes"`
	LastCheckedAt        time.Time     `bson:"lastCheckedAt" json:"lastCheckedAt"`
	LastPostGUID         string        `bson:"lastPostGuid" json:"lastPostGuid"`
	AutoPublish          bool          `bson:"autoPublish" json:"autoPublish"`
	IsActive             bool          `bson:"isActive" json:"isActive"`
	CreatedAt            time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt            time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type CreateAutolistRequest struct {
	Name            string         `json:"name" binding:"required"`
	Description     string         `json:"description"`
	TargetPlatforms []string       `json:"targetPlatforms" binding:"required"`
	TimeSlots       []AutolistSlot `json:"timeSlots" binding:"required"`
	IsCircular      bool           `json:"isCircular"`
}

type AddAutolistPostRequest struct {
	Content        string      `json:"content" binding:"required"`
	Media          []MediaItem `json:"media"`
	MaxRepetitions int         `json:"maxRepetitions"`
}

type CreateRSSFeedRequest struct {
	FeedURL              string   `json:"feedUrl" binding:"required,url"`
	TargetPlatforms      []string `json:"targetPlatforms" binding:"required"`
	Template             string   `json:"template"`
	CheckIntervalMinutes int      `json:"checkIntervalMinutes"`
	AutoPublish          bool     `json:"autoPublish"`
}
