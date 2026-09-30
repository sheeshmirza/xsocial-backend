package models

import (
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type MediaItem struct {
	Type       string  `bson:"type" json:"type"` // "image", "video", "gif"
	URL        string  `bson:"url" json:"url"`
	Base64Data string  `bson:"base64Data,omitempty" json:"base64Data,omitempty"`
	MimeType   string  `bson:"mimeType,omitempty" json:"mimeType,omitempty"`
	Size       float64 `bson:"size,omitempty" json:"size,omitempty"`
}

type PlatformPublishResponse struct {
	Platform       string `bson:"platform" json:"platform"`
	Status         string `bson:"status" json:"status"` // "success" or "failed"
	Message        string `bson:"message,omitempty" json:"message,omitempty"`
	PlatformPostID string `bson:"platformPostId,omitempty" json:"platformPostId,omitempty"`
	URL            string `bson:"url,omitempty" json:"url,omitempty"`
	Raw            any    `bson:"raw,omitempty" json:"raw,omitempty"`
	SocialHandleID string `bson:"socialHandleId,omitempty" json:"socialHandleId,omitempty"`
	PlatformID     string `bson:"platformId,omitempty" json:"platformId,omitempty"`
	AccountType    string `bson:"accountType,omitempty" json:"accountType,omitempty"`
}

type AISettings struct {
	Enabled          bool     `bson:"enabled" json:"enabled"`
	GenerationType   string   `bson:"generationType,omitempty" json:"generationType,omitempty"` // "text", "image", "both"
	TextPrompt       string   `bson:"textPrompt,omitempty" json:"textPrompt,omitempty"`
	TextSystemPrompt string   `bson:"textSystemPrompt,omitempty" json:"textSystemPrompt,omitempty"`
	TextMaxTokens    *int     `bson:"textMaxTokens,omitempty" json:"textMaxTokens,omitempty"`
	TextTemperature  *float64 `bson:"textTemperature,omitempty" json:"textTemperature,omitempty"`
	ImagePrompt      string   `bson:"imagePrompt,omitempty" json:"imagePrompt,omitempty"`
	ImageSize        string   `bson:"imageSize,omitempty" json:"imageSize,omitempty"`
	ImageCount       *int     `bson:"imageCount,omitempty" json:"imageCount,omitempty"`
	ImageQuality     string   `bson:"imageQuality,omitempty" json:"imageQuality,omitempty"`
	ImageFormat      string   `bson:"imageFormat,omitempty" json:"imageFormat,omitempty"`
}

// Common Post Domain Model
// Represents draft, scheduled (onetime / recurring), and published/posted states.
type Post struct {
	ID                 bson.ObjectID             `bson:"_id,omitempty" json:"id"`
	UserID             bson.ObjectID             `bson:"userId" json:"userId"`
	Title              string                    `bson:"title" json:"title"`
	Caption            string                    `bson:"caption" json:"caption"`
	Hashtags           []string                  `bson:"hashtags" json:"hashtags"`
	Mentions           []string                  `bson:"mentions" json:"mentions"`
	Media              []MediaItem               `bson:"media" json:"media"`
	PostType           string                    `bson:"postType" json:"postType"` // "text", "image", "video", "gif", "mixed"
	Visibility         string                    `bson:"visibility" json:"visibility"` // "public", "private"
	Timezone           string                    `bson:"timezone" json:"timezone"`
	ScheduledTime      *time.Time                `bson:"scheduledTime" json:"scheduledTime"`
	PublishedTime      *time.Time                `bson:"publishedTime" json:"publishedTime"`
	PublishedExpiresAt *time.Time                `bson:"publishedExpiresAt" json:"publishedExpiresAt"`
	Recurrence         string                    `bson:"recurrence" json:"recurrence"` // "once", "hourly", "daily", "weekly", "monthly", "yearly"
	RecurrenceInterval *int                      `bson:"recurrenceInterval" json:"recurrenceInterval"`
	RecurrenceUnit     *string                   `bson:"recurrenceUnit" json:"recurrenceUnit"`
	RecurrencePaused   bool                      `bson:"recurrencePaused" json:"recurrencePaused"`
	Status             string                    `bson:"status" json:"status"` // "draft", "scheduled", "published", "failed"
	SelectedPlatforms  []string                  `bson:"selectedPlatforms" json:"selectedPlatforms"`
	SelectedAccountIds []string                  `bson:"selectedAccountIds" json:"selectedAccountIds"`
	ExpiryMinutes      *int                      `bson:"expiryMinutes" json:"expiryMinutes"`
	ExpiryInterval     *int                      `bson:"expiryInterval" json:"expiryInterval"`
	ExpiryUnit         *string                   `bson:"expiryUnit" json:"expiryUnit"`
	PlatformResponses  []PlatformPublishResponse `bson:"platformResponses" json:"platformResponses"`
	AI                 *AISettings               `bson:"ai,omitempty" json:"ai,omitempty"`
	CreatedAt          time.Time                 `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time                 `bson:"updatedAt" json:"updatedAt"`

	// Enriched dynamically in responses
	LinkedInActivity []PostActivity `bson:"-" json:"linkedinActivity,omitempty"`
}

func (p *Post) IsDraft() bool {
	return strings.ToLower(p.Status) == PostStatusDraft
}

func (p *Post) IsScheduled() bool {
	return strings.ToLower(p.Status) == PostStatusScheduled
}

func (p *Post) IsRecurring() bool {
	return p.GetRecurrenceConfig() != nil
}

func (p *Post) IsRecurringScheduled() bool {
	return p.IsRecurring() && p.IsScheduled()
}

func (p *Post) IsPublished() bool {
	if strings.ToLower(p.Status) == PostStatusPublished {
		return true
	}
	if p.PublishedTime != nil {
		return true
	}
	for _, pr := range p.PlatformResponses {
		if pr.Status == "success" {
			return true
		}
	}
	return false
}

func (p *Post) HasSuccessfulLinkedInPublish() bool {
	for _, pr := range p.PlatformResponses {
		if strings.ToLower(pr.Platform) == "linkedin" && pr.Status == "success" && strings.TrimSpace(pr.PlatformPostID) != "" {
			return true
		}
	}
	return false
}

type RecurrenceConfig struct {
	Interval int
	Unit     string
}

func (p *Post) GetRecurrenceConfig() *RecurrenceConfig {
	if p.RecurrenceInterval != nil && p.RecurrenceUnit != nil {
		if IsValidRecurrenceInterval(*p.RecurrenceInterval, *p.RecurrenceUnit) {
			return &RecurrenceConfig{Interval: *p.RecurrenceInterval, Unit: strings.ToLower(*p.RecurrenceUnit)}
		}
	}
	normRec := strings.ToLower(strings.TrimSpace(p.Recurrence))
	switch normRec {
	case RecurrenceHourly:
		return &RecurrenceConfig{Interval: 1, Unit: "hour"}
	case RecurrenceDaily:
		return &RecurrenceConfig{Interval: 1, Unit: "day"}
	case RecurrenceWeekly:
		return &RecurrenceConfig{Interval: 1, Unit: "week"}
	case RecurrenceMonthly:
		return &RecurrenceConfig{Interval: 1, Unit: "month"}
	case RecurrenceYearly:
		return &RecurrenceConfig{Interval: 12, Unit: "month"}
	default:
		return nil
	}
}

type ExpiryConfig struct {
	Interval int
	Unit     string
}

func (p *Post) GetExpiryConfig() *ExpiryConfig {
	if p.ExpiryInterval != nil && p.ExpiryUnit != nil {
		if IsValidRecurrenceInterval(*p.ExpiryInterval, *p.ExpiryUnit) {
			return &ExpiryConfig{Interval: *p.ExpiryInterval, Unit: strings.ToLower(*p.ExpiryUnit)}
		}
	}
	if p.ExpiryMinutes != nil && *p.ExpiryMinutes >= 30 {
		return &ExpiryConfig{Interval: *p.ExpiryMinutes, Unit: "minute"}
	}
	return nil
}

func IsValidRecurrenceInterval(interval int, unit string) bool {
	bounds := map[string]struct{ min, max int }{
		"minute": {min: 30, max: 525600},
		"hour":   {min: 1, max: 8760},
		"day":    {min: 1, max: 365},
		"week":   {min: 1, max: 52},
		"month":  {min: 1, max: 12},
	}
	b, ok := bounds[strings.ToLower(unit)]
	if !ok {
		return false
	}
	return interval >= b.min && interval <= b.max
}

func RecurrenceNameForUnit(unit string) string {
	switch strings.ToLower(unit) {
	case "minute", "hour":
		return RecurrenceHourly
	case "day":
		return RecurrenceDaily
	case "week":
		return RecurrenceWeekly
	case "month":
		return RecurrenceMonthly
	default:
		return RecurrenceOnce
	}
}

// DTOs for Post Requests

type CreatePostInput struct {
	Title              string      `json:"title"`
	Caption            string      `json:"caption"`
	Hashtags           []string    `json:"hashtags"`
	Mentions           []string    `json:"mentions"`
	Media              []MediaItem `json:"media"`
	PostType           string      `json:"postType"`
	Visibility         string      `json:"visibility"`
	Timezone           string      `json:"timezone"`
	SelectedPlatforms  []string    `json:"selectedPlatforms"`
	SelectedAccountIds []string    `json:"selectedAccountIds"`
	ExpiryMinutes      *int        `json:"expiryMinutes"`
	ExpiryInterval     *int        `json:"expiryInterval"`
	ExpiryUnit         *string     `json:"expiryUnit"`
	ScheduledTime      *time.Time  `json:"scheduledTime"`
	Recurrence         string      `json:"recurrence"`
	RecurrenceInterval *int        `json:"recurrenceInterval"`
	RecurrenceUnit     *string     `json:"recurrenceUnit"`
	Status             string      `json:"status"`
	AI                 *AISettings `json:"ai"`
}

type UpdatePostInput struct {
	Title              *string      `json:"title"`
	Caption            *string      `json:"caption"`
	Hashtags           *[]string    `json:"hashtags"`
	Mentions           *[]string    `json:"mentions"`
	Media              *[]MediaItem `json:"media"`
	PostType           *string      `json:"postType"`
	Visibility         *string      `json:"visibility"`
	Timezone           *string      `json:"timezone"`
	SelectedPlatforms  *[]string    `json:"selectedPlatforms"`
	SelectedAccountIds *[]string    `json:"selectedAccountIds"`
	ExpiryMinutes      *int         `json:"expiryMinutes"`
	ExpiryInterval     *int         `json:"expiryInterval"`
	ExpiryUnit         *string      `json:"expiryUnit"`
	Recurrence         *string      `json:"recurrence"`
	RecurrenceInterval *int         `json:"recurrenceInterval"`
	RecurrenceUnit     *string      `json:"recurrenceUnit"`
	Status             *string      `json:"status"`
}

type PublishPostInput struct {
	Platforms          []string `json:"platforms"`
	AccountIds         []string `json:"accountIds"`
	Recurrence         *string  `json:"recurrence"`
	RecurrenceInterval *int     `json:"recurrenceInterval"`
	RecurrenceUnit     *string  `json:"recurrenceUnit"`
}

type SchedulePostInput struct {
	ScheduledTime      *time.Time `json:"scheduledTime"`
	Date               string     `json:"date"`
	Time               string     `json:"time"`
	Timezone           string     `json:"timezone"`
	Recurrence         string     `json:"recurrence"`
	RecurrenceInterval *int       `json:"recurrenceInterval"`
	RecurrenceUnit     *string    `json:"recurrenceUnit"`
}

type BulkIDsInput struct {
	IDs []string `json:"ids" binding:"required"`
}
