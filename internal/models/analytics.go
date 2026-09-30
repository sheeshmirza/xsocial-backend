package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type WebEvent struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	BrandID   bson.ObjectID `bson:"brandId" json:"brandId"`
	SessionID string        `bson:"sessionId" json:"sessionId"`
	URL       string        `bson:"url" json:"url"`
	Path      string        `bson:"path" json:"path"`
	Referrer  string        `bson:"referrer" json:"referrer"`
	Country   string        `bson:"country" json:"country"`
	Device    string        `bson:"device" json:"device"`
	Browser   string        `bson:"browser" json:"browser"`
	Timestamp time.Time     `bson:"timestamp" json:"timestamp"`
}

type SocialMetricRollup struct {
	ID                bson.ObjectID `bson:"_id,omitempty" json:"id"`
	BrandID           bson.ObjectID `bson:"brandId" json:"brandId"`
	Platform          string        `bson:"platform" json:"platform"`
	Date              string        `bson:"date" json:"date"` // YYYY-MM-DD
	Followers         int64         `bson:"followers" json:"followers"`
	FollowersGained   int64         `bson:"followersGained" json:"followersGained"`
	Impressions       int64         `bson:"impressions" json:"impressions"`
	Reach             int64         `bson:"reach" json:"reach"`
	Engagement        int64         `bson:"engagement" json:"engagement"`
	Clicks            int64         `bson:"clicks" json:"clicks"`
	VideoViews        int64         `bson:"videoViews" json:"videoViews"`
	PostsCount        int64         `bson:"postsCount" json:"postsCount"`
	InteractionsCount int64         `bson:"interactionsCount" json:"interactionsCount"`
	CreatedAt         time.Time     `bson:"createdAt" json:"createdAt"`
}

type BestTimeSlot struct {
	DayOfWeek int     `json:"dayOfWeek"` // 0-6 (Sun-Sat)
	Hour      int     `json:"hour"`      // 0-23
	Score     float64 `json:"score"`     // 0.0 to 100.0 activity score
}

type BestTimesResponse struct {
	Platform string         `json:"platform"`
	Timezone string         `json:"timezone"`
	Slots    []BestTimeSlot `json:"slots"`
}
