package service

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/queue"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type PostService struct {
	postRepo            repository.IPostRepository
	postActivityRepo    repository.IPostActivityRepository
	socialHandleRepo    repository.ISocialHandleRepository
	brandRepo           repository.IBrandRepository
	postPublishService  *PostPublishService
	aiService           *AIService
	activityService     *LinkedInActivityService
	socialService       *SocialService
	queue               *queue.PostQueue
}

func NewPostService(
	postRepo repository.IPostRepository,
	activityRepo repository.IPostActivityRepository,
	handleRepo repository.ISocialHandleRepository,
	brandRepo repository.IBrandRepository,
	publishService *PostPublishService,
	aiService *AIService,
	actService *LinkedInActivityService,
	socialService *SocialService,
	q *queue.PostQueue,
) *PostService {
	return &PostService{
		postRepo:           postRepo,
		postActivityRepo:   activityRepo,
		socialHandleRepo:   handleRepo,
		brandRepo:          brandRepo,
		postPublishService: publishService,
		aiService:          aiService,
		activityService:    actService,
		socialService:      socialService,
		queue:              q,
	}
}

func (s *PostService) SetQueue(q *queue.PostQueue) {
	s.queue = q
}

func (s *PostService) validateTemporaryPostDuration(interval *int, unit *string) error {
	if interval == nil && unit == nil {
		return nil
	}
	if interval != nil && unit != nil {
		if !models.IsValidRecurrenceInterval(*interval, *unit) {
			return utils.BadRequest("Temporary duration must be between 30 minutes and 12 months")
		}
	}
	return nil
}

func (s *PostService) validateInstagramContentRequirements(platforms []string, media []models.MediaItem) error {
	includesInstagram := false
	for _, p := range platforms {
		if strings.ToLower(strings.TrimSpace(p)) == "instagram" {
			includesInstagram = true
			break
		}
	}

	if includesInstagram {
		hasMedia := false
		for _, m := range media {
			if strings.TrimSpace(m.URL) != "" || strings.TrimSpace(m.Base64Data) != "" {
				hasMedia = true
				break
			}
		}
		if !hasMedia {
			return utils.BadRequest("Instagram publishing requires at least one media item (image, reel, story, or carousel)")
		}
		if len(media) > 10 {
			return utils.BadRequest("Instagram carousel allows a maximum of 10 media items")
		}
	}
	return nil
}

func (s *PostService) normalizeCaptionNewlines(text string) string {
	r := strings.ReplaceAll(text, "\r\n", "\n")
	r = strings.ReplaceAll(r, "\\r\\n", "\n")
	r = strings.ReplaceAll(r, "\\n", "\n")
	r = strings.ReplaceAll(r, "\\r", "\n")
	return strings.TrimSpace(r)
}

func (s *PostService) resolveAIGeneratedContent(ctx context.Context, payload *models.CreatePostInput) error {
	if payload.AI == nil || !payload.AI.Enabled {
		return nil
	}

	ai := payload.AI
	generationType := strings.ToLower(strings.TrimSpace(ai.GenerationType))
	if generationType == "" {
		if ai.TextPrompt != "" && ai.ImagePrompt != "" {
			generationType = "both"
		} else if ai.ImagePrompt != "" {
			generationType = "image"
		} else {
			generationType = "text"
		}
	}

	shouldGenerateText := generationType == "text" || generationType == "both"
	shouldGenerateImage := generationType == "image" || generationType == "both"

	var generatedCaption string
	if shouldGenerateText {
		textPrompt := strings.TrimSpace(ai.TextPrompt)
		if textPrompt == "" {
			return utils.BadRequest("AI text generation requires prompt")
		}

		sysPrompt := ai.TextSystemPrompt
		if sysPrompt == "" {
			sysPrompt = "You are an expert social media copywriter specializing in attention-grabbing hooks, engaging storytelling, and high-converting content. Your primary job is to take a given topic and turn it into a deeply researched, fact-checked, engaging, ready-to-post article. Don't use special characters in output."
		}

		maxTokens := 2000
		if ai.TextMaxTokens != nil && *ai.TextMaxTokens > 0 {
			maxTokens = *ai.TextMaxTokens
		}

		temp := 0.2
		if ai.TextTemperature != nil {
			temp = *ai.TextTemperature
		}

		resp, err := s.aiService.GenerateText(ctx, TextGenerationRequest{
			Provider:     "google",
			Model:        "gemini-3.1-flash-lite",
			Prompt:       textPrompt,
			SystemPrompt: sysPrompt,
			MaxTokens:    maxTokens,
			Temperature:  &temp,
			Stream:       false,
		})
		if err != nil {
			return err
		}

		generatedCaption = s.normalizeCaptionNewlines(resp.Text)
		if generatedCaption == "" && strings.TrimSpace(payload.Caption) == "" {
			return utils.BadGateway("AI text generation returned empty text")
		}
	}

	mergedMedia := append([]models.MediaItem{}, payload.Media...)
	initialMediaCount := len(mergedMedia)

	if shouldGenerateImage {
		imagePrompt := strings.TrimSpace(ai.ImagePrompt)
		if imagePrompt == "" {
			imagePrompt = strings.TrimSpace(ai.TextPrompt)
		}
		if imagePrompt == "" {
			return utils.BadRequest("AI image generation requires an image prompt")
		}

		resp, err := s.aiService.GenerateImage(ctx, ImageGenerationRequest{
			Provider: "google",
			Model:    "gemini-3.1-flash-lite",
			Prompt:   imagePrompt,
			Size:     ai.ImageSize,
			Count:    ai.ImageCount,
			Quality:  ai.ImageQuality,
			Format:   ai.ImageFormat,
		})
		if err != nil {
			return err
		}

		for _, img := range resp.Images {
			if img.URL != "" {
				mergedMedia = append(mergedMedia, models.MediaItem{
					Type:     "image",
					URL:      img.URL,
					MimeType: img.MimeType,
				})
			}
		}

		if len(mergedMedia) == initialMediaCount {
			return utils.BadGateway("AI image generation did not return image URLs")
		}
	}

	manualCaption := s.normalizeCaptionNewlines(payload.Caption)
	finalCaption := manualCaption
	if finalCaption == "" {
		finalCaption = generatedCaption
	}

	if finalCaption == "" && len(mergedMedia) == 0 {
		return utils.BadRequest("Provide caption or enable AI generation for text/image content")
	}

	inferredPostType := "text"
	if payload.Poll != nil {
		inferredPostType = "poll"
	} else if len(mergedMedia) > 0 && finalCaption != "" {
		inferredPostType = "mixed"
	} else if len(mergedMedia) > 0 {
		inferredPostType = "image"
	}

	payload.Caption = finalCaption
	payload.Media = mergedMedia
	if payload.PostType == "" {
		payload.PostType = inferredPostType
	}

	return nil
}

func (s *PostService) addMonthsClamped(d time.Time, monthsToAdd int) time.Time {
	originalDay := d.Day()
	firstOfTargetMonth := time.Date(d.Year(), d.Month()+time.Month(monthsToAdd), 1, d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), d.Location())
	lastOfTargetMonth := firstOfTargetMonth.AddDate(0, 1, -1).Day()

	targetDay := originalDay
	if targetDay > lastOfTargetMonth {
		targetDay = lastOfTargetMonth
	}

	return time.Date(firstOfTargetMonth.Year(), firstOfTargetMonth.Month(), targetDay, d.Hour(), d.Minute(), d.Second(), d.Nanosecond(), d.Location())
}

func (s *PostService) computeNextRun(currentRunAt time.Time, post *models.Post) *time.Time {
	rec := post.GetRecurrenceConfig()
	if rec == nil {
		return nil
	}

	var next time.Time
	switch rec.Unit {
	case "minute":
		next = currentRunAt.Add(time.Duration(rec.Interval) * time.Minute)
	case "hour":
		next = currentRunAt.Add(time.Duration(rec.Interval) * time.Hour)
	case "day":
		next = currentRunAt.AddDate(0, 0, rec.Interval)
	case "week":
		next = currentRunAt.AddDate(0, 0, 7*rec.Interval)
	case "month":
		next = s.addMonthsClamped(currentRunAt, rec.Interval)
	default:
		return nil
	}

	return &next
}

func (s *PostService) computeNextRunAfter(initialRunAt time.Time, post *models.Post, minTime time.Time) *time.Time {
	nextRun := initialRunAt
	guard := 0

	for (nextRun.Before(minTime) || nextRun.Equal(minTime)) && guard < 500 {
		computed := s.computeNextRun(nextRun, post)
		if computed == nil {
			return nil
		}
		nextRun = *computed
		guard++
	}

	if nextRun.After(minTime) {
		return &nextRun
	}
	return nil
}

func (s *PostService) getPublishedExpiryDate(publishedAt time.Time, post *models.Post) *time.Time {
	cfg := post.GetExpiryConfig()
	if cfg == nil {
		return nil
	}

	var exp time.Time
	switch cfg.Unit {
	case "minute":
		exp = publishedAt.Add(time.Duration(cfg.Interval) * time.Minute)
	case "hour":
		exp = publishedAt.Add(time.Duration(cfg.Interval) * time.Hour)
	case "day":
		exp = publishedAt.AddDate(0, 0, cfg.Interval)
	case "week":
		exp = publishedAt.AddDate(0, 0, 7*cfg.Interval)
	case "month":
		exp = s.addMonthsClamped(publishedAt, cfg.Interval)
	default:
		return nil
	}

	return &exp
}

func (s *PostService) enrichPostWithLinkedInActivity(ctx context.Context, userID bson.ObjectID, post *models.Post) {
	if !post.HasSuccessfulLinkedInPublish() {
		post.LinkedInActivity = []models.PostActivity{}
		return
	}

	status := strings.ToLower(post.Status)
	shouldSync := status == models.PostStatusPublished || (post.IsRecurring() && status == models.PostStatusScheduled)
	if !shouldSync {
		post.LinkedInActivity = []models.PostActivity{}
		return
	}

	activity, err := s.activityService.ListPostLinkedInActivity(ctx, userID, post.ID)
	if err == nil {
		post.LinkedInActivity = activity
	} else {
		post.LinkedInActivity = []models.PostActivity{}
	}
}

func (s *PostService) CreatePost(ctx context.Context, userID bson.ObjectID, payload models.CreatePostInput) (*models.Post, error) {
	if err := payload.Validate(); err != nil {
		return nil, utils.BadRequest(err.Error())
	}

	if err := s.resolveAIGeneratedContent(ctx, &payload); err != nil {
		return nil, err
	}

	if err := s.validateTemporaryPostDuration(payload.ExpiryInterval, payload.ExpiryUnit); err != nil {
		return nil, err
	}

	if strings.TrimSpace(payload.Caption) == "" && len(payload.Media) == 0 {
		return nil, utils.BadRequest("caption or media is required")
	}

	if err := s.validateInstagramContentRequirements(payload.SelectedPlatforms, payload.Media); err != nil {
		return nil, err
	}

	status := payload.Status
	if status == "" {
		status = models.PostStatusDraft
	}

	postType := payload.PostType
	if postType == "" {
		postType = "text"
	}

	visibility := payload.Visibility
	if visibility == "" {
		visibility = "public"
	}

	timezone := payload.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	recurrence := strings.ToLower(strings.TrimSpace(payload.Recurrence))
	if recurrence == "" {
		recurrence = models.RecurrenceOnce
	}

	if payload.BrandID != nil && !payload.BrandID.IsZero() && s.brandRepo != nil {
		member, err := s.brandRepo.FindMember(ctx, *payload.BrandID, userID)
		if err != nil || member == nil {
			return nil, utils.Forbidden("User does not have access to this brand")
		}
	}

	post := &models.Post{
		UserID:             userID,
		BrandID:            payload.BrandID,
		Title:              payload.Title,
		Caption:            payload.Caption,
		FirstComment:       payload.FirstComment,
		InstagramTrial:     payload.InstagramTrial,
		Poll:               payload.Poll,
		Hashtags:           payload.Hashtags,
		Mentions:           payload.Mentions,
		Media:              payload.Media,
		PostType:           postType,
		Visibility:         visibility,
		Timezone:           timezone,
		ScheduledTime:      payload.ScheduledTime,
		Recurrence:         recurrence,
		RecurrenceInterval: payload.RecurrenceInterval,
		RecurrenceUnit:     payload.RecurrenceUnit,
		Status:             status,
		SelectedPlatforms:  payload.SelectedPlatforms,
		SelectedAccountIds: payload.SelectedAccountIds,
		ExpiryMinutes:      payload.ExpiryMinutes,
		ExpiryInterval:     payload.ExpiryInterval,
		ExpiryUnit:         payload.ExpiryUnit,
		AI:                 payload.AI,
	}

	if err := s.postRepo.Create(ctx, post); err != nil {
		return nil, err
	}
	if post.IsScheduled() && post.ScheduledTime != nil && s.queue != nil {
		_ = s.queue.AddSchedulePublishJob(post.ID.Hex(), *post.ScheduledTime)
	}
	return post, nil
}

func (s *PostService) ListPosts(ctx context.Context, userID bson.ObjectID, queryParams map[string]string, skip, limit int, sort bson.D) ([]models.Post, int64, error) {
	// Sync LinkedIn historical activity on page 1 if not filtered by status or search
	if skip == 0 && queryParams["status"] == "" && queryParams["search"] == "" {
		go func() {
			_, _, _ = s.activityService.SyncLinkedInHistoricalPostsForAllConnectedAccounts(context.Background(), userID)
		}()
	}

	filters := bson.M{}
	if bStr := queryParams["brandId"]; bStr != "" {
		if bOID, err := bson.ObjectIDFromHex(bStr); err == nil {
			filters["brandId"] = bOID
		}
	}
	if platform := queryParams["platform"]; platform != "" {
		filters["selectedPlatforms"] = platform
	}
	if reviewStatus := queryParams["reviewStatus"]; reviewStatus != "" {
		filters["reviewStatus"] = reviewStatus
	}
	if st := queryParams["status"]; st != "" {
		filters["status"] = st
	}
	if pt := queryParams["postType"]; pt != "" {
		filters["postType"] = pt
	}

	if accIDs := queryParams["accountIds"]; accIDs != "" {
		rawList := strings.Split(accIDs, ",")
		normList := utils.NormalizeAccountIDs(rawList)
		if len(normList) > 0 {
			filters["selectedAccountIds"] = bson.M{"$in": normList}
		}
	}

	if search := queryParams["search"]; search != "" {
		escaped := regexp.QuoteMeta(search)
		filters["$or"] = []bson.M{
			{"title": bson.M{"$regex": escaped, "$options": "i"}},
			{"caption": bson.M{"$regex": escaped, "$options": "i"}},
		}
	}

	items, err := s.postRepo.ListByUserID(ctx, userID, filters, skip, limit, sort)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.postRepo.CountByUserID(ctx, userID, filters)
	if err != nil {
		return nil, 0, err
	}

	for i := range items {
		s.enrichPostWithLinkedInActivity(ctx, userID, &items[i])
	}

	return items, total, nil
}

func (s *PostService) GetPost(ctx context.Context, userID, postID bson.ObjectID) (*models.Post, error) {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return nil, utils.NotFound("Post not found")
	}

	s.enrichPostWithLinkedInActivity(ctx, userID, post)
	return post, nil
}

func (s *PostService) UpdatePost(ctx context.Context, userID, postID bson.ObjectID, input models.UpdatePostInput) (*models.Post, error) {
	if err := s.validateTemporaryPostDuration(input.ExpiryInterval, input.ExpiryUnit); err != nil {
		return nil, err
	}

	existing, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || existing == nil {
		return nil, utils.NotFound("Post not found")
	}

	nextPlatforms := existing.SelectedPlatforms
	if input.SelectedPlatforms != nil {
		nextPlatforms = *input.SelectedPlatforms
	}

	nextMedia := existing.Media
	if input.Media != nil {
		nextMedia = *input.Media
	}

	if err := s.validateInstagramContentRequirements(nextPlatforms, nextMedia); err != nil {
		return nil, err
	}

	update := bson.M{}
	if input.Title != nil {
		update["title"] = *input.Title
	}
	if input.Caption != nil {
		update["caption"] = *input.Caption
	}
	if input.Hashtags != nil {
		update["hashtags"] = *input.Hashtags
	}
	if input.Mentions != nil {
		update["mentions"] = *input.Mentions
	}
	if input.Media != nil {
		update["media"] = *input.Media
	}
	if input.PostType != nil {
		update["postType"] = *input.PostType
	}
	if input.Visibility != nil {
		update["visibility"] = *input.Visibility
	}
	if input.Timezone != nil {
		update["timezone"] = *input.Timezone
	}
	if input.SelectedPlatforms != nil {
		update["selectedPlatforms"] = *input.SelectedPlatforms
	}
	if input.SelectedAccountIds != nil {
		update["selectedAccountIds"] = *input.SelectedAccountIds
	}
	if input.ExpiryMinutes != nil {
		update["expiryMinutes"] = *input.ExpiryMinutes
	}
	if input.ExpiryInterval != nil {
		update["expiryInterval"] = *input.ExpiryInterval
	}
	if input.ExpiryUnit != nil {
		update["expiryUnit"] = *input.ExpiryUnit
	}
	if input.Recurrence != nil {
		update["recurrence"] = *input.Recurrence
	}
	if input.RecurrenceInterval != nil {
		update["recurrenceInterval"] = *input.RecurrenceInterval
	}
	if input.RecurrenceUnit != nil {
		update["recurrenceUnit"] = *input.RecurrenceUnit
	}
	if input.Status != nil {
		update["status"] = *input.Status
	}
	if input.ScheduledTime != nil {
		update["scheduledTime"] = input.ScheduledTime
	}
	if input.ReviewStatus != "" {
		update["reviewStatus"] = input.ReviewStatus
	}
	if input.ApprovalToken != "" {
		update["approvalToken"] = input.ApprovalToken
	}
	if input.FirstComment != "" {
		update["firstComment"] = input.FirstComment
	}
	if input.InstagramTrial != nil {
		update["instagramTrial"] = input.InstagramTrial
	}
	if input.Poll != nil {
		update["poll"] = input.Poll
	}
	if input.BrandID != nil {
		update["brandId"] = *input.BrandID
	}

	updated, err := s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, update)
	if err != nil {
		return nil, err
	}

	if s.queue != nil {
		// If post is no longer scheduled, cancel background publish job
		if input.Status != nil && *input.Status != models.PostStatusScheduled && existing.IsScheduled() {
			_ = s.queue.RemoveSchedulePublishJob(postID.Hex())
		}
		// If scheduled time was changed or post moved to scheduled, update background publish job
		if input.ScheduledTime != nil {
			if (input.Status != nil && *input.Status == models.PostStatusScheduled) || existing.IsScheduled() {
				_ = s.queue.AddSchedulePublishJob(postID.Hex(), *input.ScheduledTime)
			}
		}
	}

	return updated, nil
}

func (s *PostService) FindByApprovalToken(ctx context.Context, token string) (*models.Post, error) {
	return s.postRepo.FindByApprovalToken(ctx, token)
}

func (s *PostService) SubmitForApproval(ctx context.Context, userID, postID bson.ObjectID) (*models.Post, error) {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return nil, utils.NotFound("Post not found")
	}

	token := uuid.NewString()
	update := bson.M{
		"reviewStatus":  "pending",
		"approvalToken": token,
		"updatedAt":     time.Now(),
	}

	return s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, update)
}

func (s *PostService) ApprovePost(ctx context.Context, approverID, postID bson.ObjectID, comment string) (*models.Post, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil || post == nil {
		return nil, utils.NotFound("Post not found")
	}

	now := time.Now()
	update := bson.M{
		"reviewStatus": "approved",
		"approvedBy":   approverID,
		"approvedAt":   now,
		"updatedAt":    now,
	}

	if comment != "" {
		note := models.ReviewNote{
			UserID:    approverID,
			UserName:  "Approver",
			Note:      comment,
			CreatedAt: now,
		}
		if post.ReviewNotes == nil {
			update["reviewNotes"] = []models.ReviewNote{note}
		} else {
			update["reviewNotes"] = append(post.ReviewNotes, note)
		}
	}

	return s.postRepo.UpdateByID(ctx, postID, update)
}

func (s *PostService) RejectPost(ctx context.Context, rejectorID, postID bson.ObjectID, reason string) (*models.Post, error) {
	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil || post == nil {
		return nil, utils.NotFound("Post not found")
	}

	now := time.Now()
	update := bson.M{
		"reviewStatus": "rejected",
		"updatedAt":    now,
	}

	if reason != "" {
		note := models.ReviewNote{
			UserID:    rejectorID,
			UserName:  "Reviewer",
			Note:      reason,
			CreatedAt: now,
		}
		if post.ReviewNotes == nil {
			update["reviewNotes"] = []models.ReviewNote{note}
		} else {
			update["reviewNotes"] = append(post.ReviewNotes, note)
		}
	}

	return s.postRepo.UpdateByID(ctx, postID, update)
}

func (s *PostService) PublicReviewAction(ctx context.Context, token, action, reviewerName, feedback string) (*models.Post, error) {
	post, err := s.postRepo.FindByApprovalToken(ctx, token)
	if err != nil || post == nil {
		return nil, utils.NotFound("Post not found or approval link invalid")
	}

	now := time.Now()
	status := "approved"
	if strings.ToLower(action) == "reject" || strings.ToLower(action) == "rejected" {
		status = "rejected"
	}

	update := bson.M{
		"reviewStatus": status,
		"updatedAt":    now,
	}
	if status == "approved" {
		update["approvedAt"] = now
	}

	if feedback != "" || reviewerName != "" {
		note := models.ReviewNote{
			UserName:  reviewerName,
			Note:      feedback,
			CreatedAt: now,
		}
		if post.ReviewNotes == nil {
			update["reviewNotes"] = []models.ReviewNote{note}
		} else {
			update["reviewNotes"] = append(post.ReviewNotes, note)
		}
	}

	return s.postRepo.UpdateByID(ctx, post.ID, update)
}

func (s *PostService) deleteLinkedInPlatformPosts(ctx context.Context, userID bson.ObjectID, post *models.Post) {
	if post == nil {
		return
	}
	tCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for _, r := range post.PlatformResponses {
		if strings.ToLower(r.Platform) == "linkedin" && r.Status == "success" && r.PlatformPostID != "" {
			if r.SocialHandleID != "" {
				if hid, err := bson.ObjectIDFromHex(r.SocialHandleID); err == nil {
					handle, _ := s.socialHandleRepo.FindByIDAndUserID(tCtx, hid, userID)
					if handle != nil {
						fresh := s.socialService.RefreshPlatformTokenIfNeeded(tCtx, userID, handle)
						_ = s.postPublishService.linkedinService.DeletePost(tCtx, fresh.PlatformAccessToken, r.PlatformPostID)
					}
				}
			}
		}
	}
}

func (s *PostService) cleanupPostArtifacts(post *models.Post, userID bson.ObjectID) {
	if post == nil {
		return
	}
	if s.queue != nil {
		_ = s.queue.RemoveSchedulePublishJob(post.ID.Hex())
		_ = s.queue.RemovePostExpiryJobs(post.ID.Hex())
	}
	go s.deleteLinkedInPlatformPosts(context.Background(), userID, post)
}

func (s *PostService) DeletePost(ctx context.Context, userID, postID bson.ObjectID) error {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return utils.NotFound("Post not found")
	}

	s.cleanupPostArtifacts(post, userID)

	_, err = s.postRepo.DeleteByIDAndUserID(ctx, postID, userID)
	return err
}

func (s *PostService) DeleteScheduledPost(ctx context.Context, userID, postID bson.ObjectID) error {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return utils.NotFound("Post not found")
	}

	if !post.IsScheduled() {
		return utils.BadRequest("Only scheduled posts can be deleted from scheduled section")
	}

	s.cleanupPostArtifacts(post, userID)

	_, err = s.postRepo.DeleteByIDAndUserID(ctx, postID, userID)
	return err
}

func (s *PostService) DeletePublishedPost(ctx context.Context, userID, postID bson.ObjectID) error {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return utils.NotFound("Post not found")
	}

	if !post.IsPublished() {
		return utils.BadRequest("Only published posts can be deleted from published section")
	}

	if post.IsRecurringScheduled() {
		go s.deleteLinkedInPlatformPosts(context.Background(), userID, post)
		if s.queue != nil {
			_ = s.queue.RemovePostExpiryJobs(postID.Hex())
		}

		platformPostIDs := make([]string, 0)
		for _, r := range post.PlatformResponses {
			if strings.ToLower(r.Platform) == "linkedin" && r.Status == "success" {
				platformPostIDs = append(platformPostIDs, r.PlatformPostID)
			}
		}
		_ = s.postActivityRepo.DeleteByPlatformPostIDs(ctx, userID, platformPostIDs, "linkedin")

		_, err = s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, bson.M{
			"publishedTime":      nil,
			"publishedExpiresAt": nil,
			"platformResponses":  []models.PlatformPublishResponse{},
		})
		return err
	}

	s.cleanupPostArtifacts(post, userID)

	_, err = s.postRepo.DeleteByIDAndUserID(ctx, postID, userID)
	return err
}

func (s *PostService) BulkDeletePosts(ctx context.Context, userID bson.ObjectID, ids []string) (int64, error) {
	if len(ids) > 100 {
		ids = ids[:100]
	}

	posts, err := s.postRepo.FindByIDsAndUserID(ctx, ids, userID)
	if err != nil {
		return 0, err
	}

	for _, p := range posts {
		s.cleanupPostArtifacts(&p, userID)
	}

	return s.postRepo.DeleteManyByIDsAndUserID(ctx, ids, userID)
}

func (s *PostService) BulkDeleteScheduledPosts(ctx context.Context, userID bson.ObjectID, ids []string) (int64, int64, error) {
	if len(ids) > 100 {
		ids = ids[:100]
	}

	posts, err := s.postRepo.FindByIDsAndUserID(ctx, ids, userID)
	if err != nil {
		return 0, 0, err
	}

	scheduledIDs := make([]string, 0)
	for _, p := range posts {
		if p.IsScheduled() {
			scheduledIDs = append(scheduledIDs, p.ID.Hex())
			s.cleanupPostArtifacts(&p, userID)
		}
	}

	deleted, err := s.postRepo.DeleteManyByIDsAndUserID(ctx, scheduledIDs, userID)
	if err != nil {
		return 0, 0, err
	}

	skipped := int64(len(posts)) - int64(len(scheduledIDs))
	return deleted, skipped, nil
}

func (s *PostService) BulkDeletePublishedPosts(ctx context.Context, userID bson.ObjectID, ids []string) (int64, int64, error) {
	if len(ids) > 100 {
		ids = ids[:100]
	}

	posts, err := s.postRepo.FindByIDsAndUserID(ctx, ids, userID)
	if err != nil {
		return 0, 0, err
	}

	deleteIDs := make([]string, 0)
	recurringCount := int64(0)

	for _, p := range posts {
		if !p.IsPublished() {
			continue
		}

		if p.IsRecurringScheduled() {
			recurringCount++
			go s.deleteLinkedInPlatformPosts(context.Background(), userID, &p)
			if s.queue != nil {
				_ = s.queue.RemovePostExpiryJobs(p.ID.Hex())
			}
			_, _ = s.postRepo.UpdateByIDAndUserID(ctx, p.ID, userID, bson.M{
				"publishedTime":      nil,
				"publishedExpiresAt": nil,
				"platformResponses":  []models.PlatformPublishResponse{},
			})
		} else {
			deleteIDs = append(deleteIDs, p.ID.Hex())
			s.cleanupPostArtifacts(&p, userID)
		}
	}

	deleted, err := s.postRepo.DeleteManyByIDsAndUserID(ctx, deleteIDs, userID)
	if err != nil {
		return 0, 0, err
	}

	totalDeleted := deleted + recurringCount
	skipped := int64(len(posts)) - (int64(len(deleteIDs)) + recurringCount)
	return totalDeleted, skipped, nil
}

func (s *PostService) PauseRecurringPost(ctx context.Context, userID, postID bson.ObjectID) (*models.Post, error) {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return nil, utils.NotFound("Post not found")
	}

	if !post.IsRecurring() {
		return nil, utils.BadRequest("Post is not configured as recurring")
	}

	if s.queue != nil {
		_ = s.queue.RemoveSchedulePublishJob(postID.Hex())
	}

	return s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, bson.M{
		"recurrencePaused": true,
		"status":           models.PostStatusScheduled,
	})
}

func (s *PostService) computeResumedNextRun(post *models.Post, now time.Time) *time.Time {
	var nextRunAt *time.Time
	if post.ScheduledTime != nil && post.ScheduledTime.After(now) {
		nextRunAt = post.ScheduledTime
	} else if post.ScheduledTime != nil {
		nextRunAt = s.computeNextRunAfter(*post.ScheduledTime, post, now)
	} else {
		nextRunAt = s.computeNextRunAfter(now, post, now)
	}
	if nextRunAt == nil {
		nextRunAt = s.computeNextRun(now, post)
	}
	return nextRunAt
}

func (s *PostService) ResumeRecurringPost(ctx context.Context, userID, postID bson.ObjectID) (*models.Post, error) {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return nil, utils.NotFound("Post not found")
	}

	if !post.IsRecurring() {
		return nil, utils.BadRequest("Post is not configured as recurring")
	}

	now := time.Now()
	nextRunAt := s.computeResumedNextRun(post, now)
	if nextRunAt == nil || nextRunAt.Before(now) {
		return nil, utils.BadRequest("Unable to compute next recurrence run time")
	}

	updated, err := s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, bson.M{
		"recurrencePaused": false,
		"scheduledTime":    nextRunAt,
		"status":           models.PostStatusScheduled,
	})
	if err != nil {
		return nil, err
	}

	if s.queue != nil {
		_ = s.queue.AddSchedulePublishJob(postID.Hex(), *nextRunAt)
	}

	return updated, nil
}

func (s *PostService) BulkPauseRecurringPosts(ctx context.Context, userID bson.ObjectID, ids []string) (int, int, error) {
	if len(ids) > 100 {
		ids = ids[:100]
	}

	posts, err := s.postRepo.FindByIDsAndUserID(ctx, ids, userID)
	if err != nil {
		return 0, 0, err
	}

	pausedCount := 0
	for _, p := range posts {
		if p.IsRecurring() && !p.RecurrencePaused {
			if s.queue != nil {
				_ = s.queue.RemoveSchedulePublishJob(p.ID.Hex())
			}
			_, _ = s.postRepo.UpdateByIDAndUserID(ctx, p.ID, userID, bson.M{
				"recurrencePaused": true,
				"status":           models.PostStatusScheduled,
			})
			pausedCount++
		}
	}

	skipped := len(posts) - pausedCount
	return pausedCount, skipped, nil
}

func (s *PostService) BulkResumeRecurringPosts(ctx context.Context, userID bson.ObjectID, ids []string) (int, int, int, error) {
	if len(ids) > 100 {
		ids = ids[:100]
	}

	posts, err := s.postRepo.FindByIDsAndUserID(ctx, ids, userID)
	if err != nil {
		return 0, 0, 0, err
	}

	resumedCount := 0
	failedCount := 0
	eligibleCount := 0
	now := time.Now()

	for _, p := range posts {
		if p.IsRecurring() && p.RecurrencePaused {
			eligibleCount++
			nextRunAt := s.computeResumedNextRun(&p, now)
			if nextRunAt == nil || nextRunAt.Before(now) {
				failedCount++
				continue
			}

			updated, err := s.postRepo.UpdateByIDAndUserID(ctx, p.ID, userID, bson.M{
				"recurrencePaused": false,
				"scheduledTime":    nextRunAt,
				"status":           models.PostStatusScheduled,
			})
			if err == nil && updated != nil {
				resumedCount++
				if s.queue != nil {
					_ = s.queue.AddSchedulePublishJob(p.ID.Hex(), *nextRunAt)
				}
			} else {
				failedCount++
			}
		}
	}

	skipped := len(posts) - eligibleCount
	return resumedCount, skipped, failedCount, nil
}

func (s *PostService) PublishPost(
	ctx context.Context,
	userID, postID bson.ObjectID,
	platforms, accountIDs []string,
	recurrenceInput *string,
) (*models.Post, error) {
	post, err := s.GetPost(ctx, userID, postID)
	if err != nil {
		return nil, err
	}

	selectedPlatforms := platforms
	if len(selectedPlatforms) == 0 {
		selectedPlatforms = post.SelectedPlatforms
	}

	selectedAccountIDs := accountIDs
	if len(selectedAccountIDs) == 0 {
		selectedAccountIDs = post.SelectedAccountIds
	}

	if err := s.validateInstagramContentRequirements(selectedPlatforms, post.Media); err != nil {
		return nil, err
	}

	responseItems, err := s.postPublishService.PublishToPlatforms(ctx, userID, post, selectedPlatforms, selectedAccountIDs)
	if err != nil {
		return nil, err
	}

	successCount := 0
	for _, item := range responseItems {
		if item.Status == "success" {
			successCount++
		}
	}

	nextStatus := models.PostStatusPublished
	if successCount == 0 {
		nextStatus = models.PostStatusFailed
	}

	publishedAt := time.Now()
	publishedExpiresAt := s.getPublishedExpiryDate(publishedAt, post)

	recurrenceVal := post.Recurrence
	if recurrenceInput != nil {
		recurrenceVal = strings.ToLower(strings.TrimSpace(*recurrenceInput))
	}

	updated, err := s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, bson.M{
		"platformResponses":  responseItems,
		"publishedTime":      publishedAt,
		"publishedExpiresAt": publishedExpiresAt,
		"status":             nextStatus,
		"recurrencePaused":   false,
		"recurrence":         recurrenceVal,
	})
	if err != nil {
		return nil, err
	}

	if nextStatus == models.PostStatusFailed {
		msg := "Unable to publish this post to the selected accounts"
		for _, item := range responseItems {
			if item.Message != "" {
				msg = item.Message
				break
			}
		}
		return nil, utils.BadGateway(msg)
	}

	// Schedule post expiry if configured
	if nextStatus == models.PostStatusPublished && publishedExpiresAt != nil && s.queue != nil {
		for _, item := range responseItems {
			if item.Status == "success" && item.Platform != "" && item.PlatformPostID != "" && item.SocialHandleID != "" {
				_ = s.queue.AddPostExpiryJob(postID.Hex(), userID.Hex(), item.Platform, item.PlatformPostID, item.SocialHandleID, *publishedExpiresAt)
			}
		}
	}

	// Trigger LinkedIn post activity sync
	for _, item := range responseItems {
		if strings.ToLower(item.Platform) == "linkedin" && item.Status == "success" {
			go func() {
				_ = s.activityService.SyncPostLinkedInActivity(context.Background(), userID, postID)
			}()
			break
		}
	}

	// Advance recurrence if configured
	if nextStatus == models.PostStatusPublished && recurrenceVal != models.RecurrenceOnce {
		nextRunAt := s.computeNextRun(publishedAt, updated)
		if nextRunAt != nil {
			recurrentPost, err := s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, bson.M{
				"scheduledTime":    nextRunAt,
				"recurrence":       recurrenceVal,
				"recurrencePaused": false,
				"status":           models.PostStatusScheduled,
			})
			if err == nil && recurrentPost != nil {
				if s.queue != nil {
					_ = s.queue.AddSchedulePublishJob(postID.Hex(), *nextRunAt)
				}
				return recurrentPost, nil
			}
		}
	}

	return updated, nil
}

func (s *PostService) SchedulePost(ctx context.Context, userID, postID bson.ObjectID, input models.SchedulePostInput) (*models.Post, error) {
	post, err := s.GetPost(ctx, userID, postID)
	if err != nil {
		return nil, err
	}

	if err := s.validateInstagramContentRequirements(post.SelectedPlatforms, post.Media); err != nil {
		return nil, err
	}

	var scheduledTime time.Time
	if input.ScheduledTime != nil {
		scheduledTime = *input.ScheduledTime
	} else if input.Date != "" && input.Time != "" {
		tz := input.Timezone
		if tz == "" {
			tz = "UTC"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return nil, utils.BadRequest("Invalid timezone value for scheduling")
		}

		dateTimeStr := fmt.Sprintf("%s %s", input.Date, input.Time)
		parsed, err := time.ParseInLocation("2006-01-02 15:04", dateTimeStr, loc)
		if err != nil {
			parsed, err = time.ParseInLocation("2006-01-02 15:04:05", dateTimeStr, loc)
			if err != nil {
				return nil, utils.BadRequest("Invalid date/time values for scheduling")
			}
		}
		scheduledTime = parsed.UTC()
	} else {
		return nil, utils.BadRequest("Provide scheduledTime or both date and time")
	}

	if scheduledTime.Before(time.Now()) {
		return nil, utils.BadRequest("Scheduled time must be in the future")
	}

	hasCustomRecurrence := input.RecurrenceInterval != nil || input.RecurrenceUnit != nil
	if hasCustomRecurrence {
		if input.RecurrenceInterval == nil || input.RecurrenceUnit == nil || !models.IsValidRecurrenceInterval(*input.RecurrenceInterval, *input.RecurrenceUnit) {
			return nil, utils.BadRequest("Recurrence must be between 30 minutes and 12 months")
		}
	}

	recurrence := input.Recurrence
	if hasCustomRecurrence {
		recurrence = models.RecurrenceNameForUnit(*input.RecurrenceUnit)
	} else if recurrence == "" {
		recurrence = models.RecurrenceOnce
	}

	timezone := input.Timezone
	if timezone == "" {
		timezone = post.Timezone
	}

	update := bson.M{
		"scheduledTime":      scheduledTime,
		"timezone":           timezone,
		"recurrence":         recurrence,
		"recurrenceInterval": input.RecurrenceInterval,
		"recurrenceUnit":     input.RecurrenceUnit,
		"recurrencePaused":   false,
		"status":             models.PostStatusScheduled,
	}

	updated, err := s.postRepo.UpdateByIDAndUserID(ctx, postID, userID, update)
	if err != nil {
		return nil, err
	}

	if s.queue != nil {
		_ = s.queue.AddSchedulePublishJob(postID.Hex(), scheduledTime)
	}

	return updated, nil
}

func (s *PostService) createRecurringOccurrence(ctx context.Context, templatePost *models.Post) (*models.Post, error) {
	occurrence := &models.Post{
		UserID:             templatePost.UserID,
		AI:                 templatePost.AI,
		Title:              templatePost.Title,
		Caption:            templatePost.Caption,
		Hashtags:           templatePost.Hashtags,
		Mentions:           templatePost.Mentions,
		Media:              templatePost.Media,
		PostType:           templatePost.PostType,
		Visibility:         templatePost.Visibility,
		Timezone:           templatePost.Timezone,
		ScheduledTime:      templatePost.ScheduledTime,
		Recurrence:         models.RecurrenceOnce,
		Status:             models.PostStatusScheduled,
		SelectedPlatforms:  templatePost.SelectedPlatforms,
		SelectedAccountIds: templatePost.SelectedAccountIds,
		ExpiryMinutes:      templatePost.ExpiryMinutes,
		ExpiryInterval:     templatePost.ExpiryInterval,
		ExpiryUnit:         templatePost.ExpiryUnit,
	}

	if templatePost.AI != nil && templatePost.AI.Enabled {
		input := models.CreatePostInput{
			AI:                 templatePost.AI,
			Title:              templatePost.Title,
			Hashtags:           templatePost.Hashtags,
			Mentions:           templatePost.Mentions,
			PostType:           templatePost.PostType,
			Visibility:         templatePost.Visibility,
			Timezone:           templatePost.Timezone,
			SelectedPlatforms:  templatePost.SelectedPlatforms,
			SelectedAccountIds: templatePost.SelectedAccountIds,
		}
		if err := s.resolveAIGeneratedContent(ctx, &input); err == nil {
			occurrence.Caption = input.Caption
			occurrence.Media = input.Media
			occurrence.PostType = input.PostType
		}
	}

	if err := s.postRepo.Create(ctx, occurrence); err != nil {
		return nil, err
	}
	return occurrence, nil
}

func (s *PostService) PublishScheduledPost(ctx context.Context, postIDStr string) error {
	postID, err := bson.ObjectIDFromHex(postIDStr)
	if err != nil {
		return err
	}

	post, err := s.postRepo.FindByID(ctx, postID)
	if err != nil || post == nil {
		return nil
	}

	isRecurring := post.IsRecurring()
	shouldAdvanceRecurrence := false

	if isRecurring {
		occurrence, err := s.createRecurringOccurrence(ctx, post)
		if err != nil {
			log.Printf("[PostService] Recurring occurrence creation failed for %s: %v", postIDStr, err)
		} else {
			_, _ = s.PublishPost(ctx, post.UserID, occurrence.ID, post.SelectedPlatforms, post.SelectedAccountIds, nil)
		}
		shouldAdvanceRecurrence = true
	} else {
		published, err := s.PublishPost(ctx, post.UserID, post.ID, post.SelectedPlatforms, post.SelectedAccountIds, nil)
		if err == nil && published != nil && published.Status == models.PostStatusPublished {
			shouldAdvanceRecurrence = false
		}
	}

	if shouldAdvanceRecurrence {
		baseRunAt := time.Now()
		if post.ScheduledTime != nil {
			baseRunAt = *post.ScheduledTime
		}
		nextRunAt := s.computeNextRun(baseRunAt, post)
		if nextRunAt != nil {
			_, _ = s.postRepo.UpdateByIDAndUserID(ctx, post.ID, post.UserID, bson.M{
				"scheduledTime":    nextRunAt,
				"recurrencePaused": false,
				"status":           models.PostStatusScheduled,
			})
			if s.queue != nil {
				_ = s.queue.AddSchedulePublishJob(post.ID.Hex(), *nextRunAt)
			}
		}
	}

	return nil
}

func (s *PostService) ExpirePublishedPlatformPost(ctx context.Context, payload queue.ExpiryJobData) error {
	postID, err := bson.ObjectIDFromHex(payload.PostID)
	if err != nil {
		return err
	}
	userOID, err := bson.ObjectIDFromHex(payload.UserID)
	if err != nil {
		return err
	}

	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userOID)
	if err != nil || post == nil {
		return nil
	}

	platform := strings.ToLower(strings.TrimSpace(payload.Platform))
	if platform == "linkedin" && payload.SocialHandleID != "" {
		if hid, err := bson.ObjectIDFromHex(payload.SocialHandleID); err == nil {
			handle, _ := s.socialHandleRepo.FindByIDAndUserID(ctx, hid, userOID)
			if handle != nil {
				fresh := s.socialService.RefreshPlatformTokenIfNeeded(ctx, userOID, handle)
				_ = s.postPublishService.linkedinService.DeletePost(ctx, fresh.PlatformAccessToken, payload.PlatformPostID)
			}
		}
		_ = s.postActivityRepo.DeleteByPostPlatformAndPlatformPostID(ctx, userOID, "linkedin", payload.PlatformPostID)
	}

	remainingResponses := make([]models.PlatformPublishResponse, 0)
	for _, item := range post.PlatformResponses {
		if strings.ToLower(item.Platform) == platform && item.PlatformPostID == payload.PlatformPostID {
			continue
		}
		remainingResponses = append(remainingResponses, item)
	}

	if len(remainingResponses) == len(post.PlatformResponses) {
		return nil
	}

	if post.IsRecurring() {
		update := bson.M{
			"platformResponses": remainingResponses,
		}
		if len(remainingResponses) == 0 {
			update["publishedTime"] = nil
			update["publishedExpiresAt"] = nil
		}
		_, err := s.postRepo.UpdateByIDAndUserID(ctx, postID, userOID, update)
		return err
	}

	if s.queue != nil {
		_ = s.queue.RemovePostExpiryJobs(postID.Hex())
	}
	_, err = s.postRepo.DeleteByIDAndUserID(ctx, postID, userOID)
	return err
}

func (s *PostService) RequeueRecurringScheduledPostsOnStartup(ctx context.Context) {
	posts, err := s.postRepo.ListRecurringScheduledPosts(ctx)
	if err != nil || len(posts) == 0 {
		return
	}

	restoredCount := 0
	for _, p := range posts {
		if p.ScheduledTime == nil {
			continue
		}
		if s.queue != nil {
			_ = s.queue.AddSchedulePublishJob(p.ID.Hex(), *p.ScheduledTime)
			restoredCount++
		}
	}
	log.Printf("[PostService] Restored %d recurring scheduled posts on startup", restoredCount)
}

func (s *PostService) StatusBreakdown(ctx context.Context, userID bson.ObjectID, accountIDs []string) (map[string]int64, error) {
	filters := bson.M{}
	if len(accountIDs) > 0 {
		filters["selectedAccountIds"] = bson.M{"$in": accountIDs}
	}
	return s.postRepo.AggregateDashboard(ctx, userID, filters)
}

func (s *PostService) PlatformBreakdown(ctx context.Context, userID bson.ObjectID, accountIDs []string) ([]repository.PlatformStatsResult, error) {
	filters := bson.M{}
	if len(accountIDs) > 0 {
		filters["selectedAccountIds"] = bson.M{"$in": accountIDs}
	}
	return s.postRepo.PlatformStats(ctx, userID, filters)
}

func (s *PostService) ResharePost(ctx context.Context, userID, sourcePostID bson.ObjectID, req models.ResharePostRequest) (*models.Post, error) {
	// 1. Fetch source post
	sourcePost, err := s.postRepo.FindByIDAndUserID(ctx, sourcePostID, userID)
	if err != nil || sourcePost == nil {
		return nil, utils.NotFound("Source post not found")
	}

	// 2. Validate target accounts
	if len(req.TargetAccountIDs) == 0 && len(req.TargetPlatforms) == 0 {
		return nil, utils.BadRequest("Target account IDs or target platforms must be specified for reshare")
	}

	var targetHandles []models.SocialHandle
	if len(req.TargetAccountIDs) > 0 {
		handles, hErr := s.socialHandleRepo.FindByUserAndIDs(ctx, userID, req.TargetAccountIDs)
		if hErr != nil || len(handles) == 0 {
			return nil, utils.BadRequest("Invalid target accounts or accounts not connected to this user")
		}
		targetHandles = handles
	} else {
		handles, hErr := s.socialHandleRepo.FindByUserPlatforms(ctx, userID, req.TargetPlatforms)
		if hErr != nil || len(handles) == 0 {
			return nil, utils.BadRequest("No connected accounts found for specified target platforms")
		}
		targetHandles = handles
	}

	targetPlatforms := make([]string, 0, len(targetHandles))
	targetAccountIDs := make([]string, 0, len(targetHandles))
	for _, h := range targetHandles {
		targetPlatforms = append(targetPlatforms, strings.ToLower(h.Platform))
		targetAccountIDs = append(targetAccountIDs, h.ID.Hex())
	}

	// 3. Resolve caption and commentary
	finalCaption := sourcePost.Caption
	reshareCommentary := ""
	if req.NewCaption != nil && strings.TrimSpace(*req.NewCaption) != "" {
		finalCaption = strings.TrimSpace(*req.NewCaption)
		reshareCommentary = finalCaption
	}

	// 4. Resolve media
	var media []models.MediaItem
	if req.IncludeMedia == nil || *req.IncludeMedia {
		media = append([]models.MediaItem{}, sourcePost.Media...)
	}

	// 5. Origin post ID and origin platform post ID
	originPlatformPostID := ""
	originAccountID := ""
	for _, resp := range sourcePost.PlatformResponses {
		if resp.Status == "success" && resp.PlatformPostID != "" {
			originPlatformPostID = resp.PlatformPostID
			originAccountID = resp.SocialHandleID
			break
		}
	}

	status := models.PostStatusDraft
	if req.ScheduledTime != nil && req.ScheduledTime.After(time.Now()) {
		status = models.PostStatusScheduled
	}

	resharePost := &models.Post{
		UserID:               userID,
		BrandID:              sourcePost.BrandID,
		Title:                sourcePost.Title,
		Caption:              finalCaption,
		Hashtags:             sourcePost.Hashtags,
		Mentions:             sourcePost.Mentions,
		Media:                media,
		PostType:             sourcePost.PostType,
		Visibility:           sourcePost.Visibility,
		Timezone:             sourcePost.Timezone,
		ScheduledTime:        req.ScheduledTime,
		Status:               status,
		SelectedPlatforms:    targetPlatforms,
		SelectedAccountIds:   targetAccountIDs,
		OriginPostID:         &sourcePost.ID,
		OriginPlatformPostID: originPlatformPostID,
		OriginAccountID:      originAccountID,
		IsReshare:            true,
		ReshareCommentary:    reshareCommentary,
		FirstComment:         sourcePost.FirstComment,
	}

	if err := s.postRepo.Create(ctx, resharePost); err != nil {
		return nil, utils.InternalServerError("Failed to create reshare post")
	}

	if resharePost.IsScheduled() && resharePost.ScheduledTime != nil && s.queue != nil {
		_ = s.queue.AddSchedulePublishJob(resharePost.ID.Hex(), *resharePost.ScheduledTime)
	}

	// 6. If immediate publication requested
	if req.PublishImmediately {
		pubResponses, pubErr := s.postPublishService.PublishToPlatforms(ctx, userID, resharePost, targetPlatforms, targetAccountIDs)
		if pubErr != nil {
			return resharePost, pubErr
		}
		resharePost.PlatformResponses = pubResponses

		hasSuccess := false
		for _, r := range pubResponses {
			if r.Status == "success" {
				hasSuccess = true
				break
			}
		}

		now := time.Now()
		update := bson.M{
			"platformResponses": pubResponses,
			"updatedAt":         now,
		}
		if hasSuccess {
			resharePost.Status = models.PostStatusPublished
			resharePost.PublishedTime = &now
			update["status"] = models.PostStatusPublished
			update["publishedTime"] = now
		} else {
			resharePost.Status = models.PostStatusFailed
			update["status"] = models.PostStatusFailed
		}
		_, _ = s.postRepo.UpdateByIDAndUserID(ctx, resharePost.ID, userID, update)
	}

	return resharePost, nil
}

