package models

import (
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type MediaItem struct {
	Type       string  `bson:"type" json:"type"` // "image", "video", "gif"
	URL        string  `bson:"url" json:"url"`
	Base64Data string  `bson:"-" json:"base64Data,omitempty"`
	MimeType   string  `bson:"mimeType,omitempty" json:"mimeType,omitempty"`
	Size       float64 `bson:"size,omitempty" json:"size,omitempty"`
}

type ReviewNote struct {
	UserID    bson.ObjectID `bson:"userId" json:"userId"`
	UserName  string        `bson:"userName" json:"userName"`
	Note      string        `bson:"note" json:"note"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
}

type PlatformPublishResponse struct {
	Platform       string `bson:"platform" json:"platform"`
	Status         string `bson:"status" json:"status"` // "success" or "failed"
	Message        string `bson:"message,omitempty" json:"message,omitempty"`
	PlatformPostID string `bson:"platformPostId,omitempty" json:"platformPostId,omitempty"`
	URL            string `bson:"url,omitempty" json:"url,omitempty"`
	Raw            any    `bson:"-" json:"raw,omitempty"`
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

type InstagramTrialSettings struct {
	Enabled            bool   `bson:"enabled" json:"enabled"`
	GraduationStrategy string `bson:"graduationStrategy,omitempty" json:"graduationStrategy,omitempty"` // "MANUAL" or "SS_PERFORMANCE"
}

type PollOption struct {
	Text string `bson:"text" json:"text"`
}

type PollSettings struct {
	Duration          string `bson:"duration" json:"duration"` // "ONE_DAY", "THREE_DAYS", "SEVEN_DAYS", "FOURTEEN_DAYS"
	VoteSelectionType string `bson:"voteSelectionType,omitempty" json:"voteSelectionType,omitempty"` // "SINGLE_VOTE"
}

type PollData struct {
	Question string       `bson:"question" json:"question"`
	Options  []PollOption `bson:"options" json:"options"`
	Settings PollSettings `bson:"settings" json:"settings"`
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
	PostType           string                    `bson:"postType" json:"postType"` // "text", "image", "video", "gif", "mixed", "reel", "story", "carousel", "poll", "article"
	Poll               *PollData                 `bson:"poll,omitempty" json:"poll,omitempty"`
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
	BrandID            *bson.ObjectID            `bson:"brandId,omitempty" json:"brandId,omitempty"`
	ReviewStatus       string                    `bson:"reviewStatus,omitempty" json:"reviewStatus,omitempty"`
	ReviewNotes        []ReviewNote              `bson:"reviewNotes,omitempty" json:"reviewNotes,omitempty"`
	ApprovalToken      string                    `bson:"approvalToken,omitempty" json:"approvalToken,omitempty"`
	ApprovedBy         *bson.ObjectID            `bson:"approvedBy,omitempty" json:"approvedBy,omitempty"`
	ApprovedAt         *time.Time                `bson:"approvedAt,omitempty" json:"approvedAt,omitempty"`
	FirstComment       string                    `bson:"firstComment,omitempty" json:"firstComment,omitempty"`
	InstagramTrial     *InstagramTrialSettings   `bson:"instagramTrial,omitempty" json:"instagramTrial,omitempty"`
	OriginPostID       *bson.ObjectID            `bson:"originPostId,omitempty" json:"originPostId,omitempty"`
	OriginPlatformPostID string                  `bson:"originPlatformPostId,omitempty" json:"originPlatformPostId,omitempty"`
	OriginAccountID    string                    `bson:"originAccountId,omitempty" json:"originAccountId,omitempty"`
	IsReshare          bool                      `bson:"isReshare,omitempty" json:"isReshare,omitempty"`
	ReshareCommentary  string                    `bson:"reshareCommentary,omitempty" json:"reshareCommentary,omitempty"`
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
	RecurrenceUnit     *string        `json:"recurrenceUnit"`
	Status             string                  `json:"status"`
	BrandID            *bson.ObjectID          `json:"brandId,omitempty"`
	FirstComment       string                  `json:"firstComment,omitempty"`
	InstagramTrial     *InstagramTrialSettings `json:"instagramTrial,omitempty"`
	Poll               *PollData               `json:"poll,omitempty"`
	AI                 *AISettings             `json:"ai"`
}

func (in *CreatePostInput) Validate() error {
	in.Title = strings.TrimSpace(in.Title)
	if len(in.Title) > 500 {
		return fmt.Errorf("title exceeds maximum allowed length of 500 characters")
	}
	in.Caption = strings.TrimSpace(in.Caption)
	if len(in.Caption) > 50000 {
		return fmt.Errorf("caption exceeds maximum allowed length of 50,000 characters")
	}
	if in.PostType != "" {
		validTypes := map[string]bool{
			"text": true, "image": true, "video": true, "gif": true,
			"mixed": true, "reel": true, "story": true, "carousel": true,
			"poll": true, "article": true,
		}
		if !validTypes[strings.ToLower(in.PostType)] {
			return fmt.Errorf("invalid postType: %s", in.PostType)
		}
	}
	if in.Visibility != "" {
		v := strings.ToLower(in.Visibility)
		if v != "public" && v != "private" {
			return fmt.Errorf("visibility must be either 'public' or 'private'")
		}
	}
	if in.Timezone != "" && in.Timezone != "UTC" {
		if _, err := time.LoadLocation(in.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", in.Timezone)
		}
	}
	if in.Recurrence != "" {
		validRec := map[string]bool{
			RecurrenceOnce: true, RecurrenceHourly: true, RecurrenceDaily: true,
			RecurrenceWeekly: true, RecurrenceMonthly: true, RecurrenceYearly: true,
		}
		if !validRec[strings.ToLower(in.Recurrence)] {
			return fmt.Errorf("invalid recurrence: %s", in.Recurrence)
		}
	}
	for i, m := range in.Media {
		if m.URL != "" {
			trimmedURL := strings.TrimSpace(m.URL)
			if !strings.HasPrefix(trimmedURL, "http://") && !strings.HasPrefix(trimmedURL, "https://") {
				return fmt.Errorf("media[%d] URL must start with http:// or https://", i)
			}
		}
	}
	if in.Poll != nil {
		if len(in.Poll.Options) < 2 {
			return fmt.Errorf("poll requires at least 2 options")
		}
		if len(in.Poll.Options) > 10 {
			return fmt.Errorf("poll cannot have more than 10 options")
		}
	}
	return nil
}

type UpdatePostInput struct {
	Title              *string                 `json:"title"`
	Caption            *string                 `json:"caption"`
	Hashtags           *[]string               `json:"hashtags"`
	Mentions           *[]string               `json:"mentions"`
	Media              *[]MediaItem            `json:"media"`
	PostType           *string                 `json:"postType"`
	Visibility         *string                 `json:"visibility"`
	Timezone           *string                 `json:"timezone"`
	SelectedPlatforms  *[]string               `json:"selectedPlatforms"`
	SelectedAccountIds *[]string               `json:"selectedAccountIds"`
	ExpiryMinutes      *int                    `json:"expiryMinutes"`
	ExpiryInterval     *int                    `json:"expiryInterval"`
	ExpiryUnit         *string                 `json:"expiryUnit"`
	Recurrence         *string                 `json:"recurrence"`
	RecurrenceInterval *int                    `json:"recurrenceInterval"`
	RecurrenceUnit     *string                 `json:"recurrenceUnit"`
	Status             *string                 `json:"status"`
	ScheduledTime      *time.Time              `json:"scheduledTime,omitempty"`
	BrandID            *bson.ObjectID          `json:"brandId"`
	ReviewStatus       string                  `json:"reviewStatus"`
	ApprovalToken      string                  `json:"approvalToken"`
	FirstComment       string                  `json:"firstComment"`
	InstagramTrial     *InstagramTrialSettings `json:"instagramTrial,omitempty"`
	Poll               *PollData               `json:"poll,omitempty"`
}

func (in *UpdatePostInput) Validate() error {
	if in.Title != nil {
		*in.Title = strings.TrimSpace(*in.Title)
		if len(*in.Title) > 500 {
			return fmt.Errorf("title exceeds maximum allowed length of 500 characters")
		}
	}
	if in.Caption != nil {
		*in.Caption = strings.TrimSpace(*in.Caption)
		if len(*in.Caption) > 50000 {
			return fmt.Errorf("caption exceeds maximum allowed length of 50,000 characters")
		}
	}
	if in.PostType != nil && *in.PostType != "" {
		validTypes := map[string]bool{
			"text": true, "image": true, "video": true, "gif": true,
			"mixed": true, "reel": true, "story": true, "carousel": true,
			"poll": true, "article": true,
		}
		if !validTypes[strings.ToLower(*in.PostType)] {
			return fmt.Errorf("invalid postType: %s", *in.PostType)
		}
	}
	if in.Visibility != nil && *in.Visibility != "" {
		v := strings.ToLower(*in.Visibility)
		if v != "public" && v != "private" {
			return fmt.Errorf("visibility must be either 'public' or 'private'")
		}
	}
	if in.Timezone != nil && *in.Timezone != "" && *in.Timezone != "UTC" {
		if _, err := time.LoadLocation(*in.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", *in.Timezone)
		}
	}
	if in.Recurrence != nil && *in.Recurrence != "" {
		validRec := map[string]bool{
			RecurrenceOnce: true, RecurrenceHourly: true, RecurrenceDaily: true,
			RecurrenceWeekly: true, RecurrenceMonthly: true, RecurrenceYearly: true,
		}
		if !validRec[strings.ToLower(*in.Recurrence)] {
			return fmt.Errorf("invalid recurrence: %s", *in.Recurrence)
		}
	}
	if in.Media != nil {
		for i, m := range *in.Media {
			if m.URL != "" {
				trimmedURL := strings.TrimSpace(m.URL)
				if !strings.HasPrefix(trimmedURL, "http://") && !strings.HasPrefix(trimmedURL, "https://") {
					return fmt.Errorf("media[%d] URL must start with http:// or https://", i)
				}
			}
		}
	}
	if in.Poll != nil {
		if len(in.Poll.Options) < 2 {
			return fmt.Errorf("poll requires at least 2 options")
		}
		if len(in.Poll.Options) > 10 {
			return fmt.Errorf("poll cannot have more than 10 options")
		}
	}
	return nil
}

type CreatePostRequest = CreatePostInput
type UpdatePostRequest = UpdatePostInput

type PublishPostInput struct {
	Platforms          []string `json:"platforms"`
	AccountIds         []string `json:"accountIds"`
	Recurrence         *string  `json:"recurrence"`
	RecurrenceInterval *int     `json:"recurrenceInterval"`
	RecurrenceUnit     *string  `json:"recurrenceUnit"`
}

func (in *PublishPostInput) Validate() error {
	validPlatforms := map[string]bool{
		strings.ToLower(PlatformInstagram): true,
		strings.ToLower(PlatformLinkedIn):  true,
	}
	for i, p := range in.Platforms {
		norm := strings.ToLower(strings.TrimSpace(p))
		in.Platforms[i] = norm
		if !validPlatforms[norm] {
			return fmt.Errorf("unsupported platform: %s", p)
		}
	}
	if in.Recurrence != nil && *in.Recurrence != "" {
		normRec := strings.ToLower(strings.TrimSpace(*in.Recurrence))
		validRec := map[string]bool{
			RecurrenceOnce: true, RecurrenceHourly: true, RecurrenceDaily: true,
			RecurrenceWeekly: true, RecurrenceMonthly: true, RecurrenceYearly: true,
		}
		if !validRec[normRec] {
			return fmt.Errorf("invalid recurrence: %s", *in.Recurrence)
		}
	}
	if in.RecurrenceInterval != nil && in.RecurrenceUnit != nil {
		if !IsValidRecurrenceInterval(*in.RecurrenceInterval, *in.RecurrenceUnit) {
			return fmt.Errorf("invalid recurrence interval %d for unit %s", *in.RecurrenceInterval, *in.RecurrenceUnit)
		}
	}
	return nil
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

func (in *SchedulePostInput) Validate() error {
	if in.Timezone != "" && in.Timezone != "UTC" {
		if _, err := time.LoadLocation(in.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", in.Timezone)
		}
	}
	if in.Recurrence != "" {
		validRec := map[string]bool{
			RecurrenceOnce: true, RecurrenceHourly: true, RecurrenceDaily: true,
			RecurrenceWeekly: true, RecurrenceMonthly: true, RecurrenceYearly: true,
		}
		if !validRec[strings.ToLower(in.Recurrence)] {
			return fmt.Errorf("invalid recurrence: %s", in.Recurrence)
		}
	}
	if in.RecurrenceInterval != nil && in.RecurrenceUnit != nil {
		if !IsValidRecurrenceInterval(*in.RecurrenceInterval, *in.RecurrenceUnit) {
			return fmt.Errorf("invalid recurrence interval %d for unit %s", *in.RecurrenceInterval, *in.RecurrenceUnit)
		}
	}
	return nil
}

type ApprovePostRequest struct {
	Comment string `json:"comment"`
}

func (r *ApprovePostRequest) Validate() error {
	r.Comment = strings.ReplaceAll(r.Comment, "\x00", "")
	r.Comment = strings.TrimSpace(r.Comment)
	if len(r.Comment) > 2000 {
		return fmt.Errorf("comment exceeds maximum allowed length of 2,000 characters")
	}
	return nil
}

type RejectPostRequest struct {
	Reason string `json:"reason"`
}

func (r *RejectPostRequest) Validate() error {
	r.Reason = strings.ReplaceAll(r.Reason, "\x00", "")
	r.Reason = strings.TrimSpace(r.Reason)
	if len(r.Reason) > 2000 {
		return fmt.Errorf("reason exceeds maximum allowed length of 2,000 characters")
	}
	return nil
}

type PublicReviewActionRequest struct {
	Action       string `json:"action"` // "approve" or "reject"
	ReviewerName string `json:"reviewerName"`
	Feedback     string `json:"feedback"`
}

func (r *PublicReviewActionRequest) Validate() error {
	r.Action = strings.ToLower(strings.TrimSpace(r.Action))
	if r.Action != "approve" && r.Action != "reject" {
		return fmt.Errorf("action must be either 'approve' or 'reject'")
	}
	r.ReviewerName = strings.ReplaceAll(r.ReviewerName, "\x00", "")
	r.ReviewerName = strings.TrimSpace(r.ReviewerName)
	if len(r.ReviewerName) > 100 {
		return fmt.Errorf("reviewer name must not exceed 100 characters")
	}
	r.Feedback = strings.ReplaceAll(r.Feedback, "\x00", "")
	r.Feedback = strings.TrimSpace(r.Feedback)
	if len(r.Feedback) > 2000 {
		return fmt.Errorf("feedback exceeds maximum allowed length of 2,000 characters")
	}
	return nil
}

type BulkIDsInput struct {
	IDs []string `json:"ids" binding:"required"`
}

func (in *BulkIDsInput) Validate() error {
	if len(in.IDs) == 0 {
		return fmt.Errorf("at least one ID is required")
	}
	if len(in.IDs) > 100 {
		return fmt.Errorf("bulk operation limit is 100 IDs per request")
	}
	return nil
}

type ResharePostRequest struct {
	TargetAccountIDs   []string   `json:"targetAccountIds"`
	TargetPlatforms    []string   `json:"targetPlatforms"`
	NewCaption         *string    `json:"newCaption"`
	ScheduledTime      *time.Time `json:"scheduledTime"`
	IncludeMedia       *bool      `json:"includeMedia"`
	PublishImmediately bool       `json:"publishImmediately"`
}

func (r *ResharePostRequest) Validate() error {
	if len(r.TargetAccountIDs) == 0 && len(r.TargetPlatforms) == 0 {
		return fmt.Errorf("target account IDs or target platforms must be specified for reshare")
	}
	if r.NewCaption != nil {
		*r.NewCaption = strings.TrimSpace(*r.NewCaption)
		if len(*r.NewCaption) > 50000 {
			return fmt.Errorf("caption exceeds maximum allowed length of 50,000 characters")
		}
	}
	if r.ScheduledTime != nil && !r.PublishImmediately && r.ScheduledTime.Before(time.Now().Add(-5*time.Minute)) {
		return fmt.Errorf("scheduled time cannot be in the past")
	}
	return nil
}

