package service

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type LinkedInActivityService struct {
	postRepo         repository.IPostRepository
	socialHandleRepo repository.ISocialHandleRepository
	postActivityRepo repository.IPostActivityRepository
	linkedinService  *linkedin.Service
}

func NewLinkedInActivityService(
	postRepo repository.IPostRepository,
	handleRepo repository.ISocialHandleRepository,
	activityRepo repository.IPostActivityRepository,
	liService *linkedin.Service,
) *LinkedInActivityService {
	return &LinkedInActivityService{
		postRepo:         postRepo,
		socialHandleRepo: handleRepo,
		postActivityRepo: activityRepo,
		linkedinService:  liService,
	}
}

func (s *LinkedInActivityService) buildContentSnapshot(post *models.Post) models.ContentSnapshot {
	media := make([]models.MediaSnapshot, 0, len(post.Media))
	for _, m := range post.Media {
		media = append(media, models.MediaSnapshot{
			Type:     m.Type,
			URL:      m.URL,
			MimeType: m.MimeType,
		})
	}

	return models.ContentSnapshot{
		Title:              post.Title,
		Caption:            post.Caption,
		Hashtags:           post.Hashtags,
		Mentions:           post.Mentions,
		Media:              media,
		PublishedTime:      post.PublishedTime,
		PublishedExpiresAt: post.PublishedExpiresAt,
	}
}

func (s *LinkedInActivityService) SyncPostLinkedInActivity(ctx context.Context, userID, postID bson.ObjectID) error {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return utils.NotFound("Post not found")
	}

	linkedInResponses := make([]models.PlatformPublishResponse, 0)
	for _, r := range post.PlatformResponses {
		if strings.ToLower(r.Platform) == "linkedin" && r.Status == "success" && strings.TrimSpace(r.PlatformPostID) != "" {
			linkedInResponses = append(linkedInResponses, r)
		}
	}

	if len(linkedInResponses) == 0 {
		return utils.BadRequest("No successful LinkedIn publish found for this post")
	}

	for _, respItem := range linkedInResponses {
		var handle *models.SocialHandle
		if respItem.SocialHandleID != "" {
			if hid, err := bson.ObjectIDFromHex(respItem.SocialHandleID); err == nil {
				handle, _ = s.socialHandleRepo.FindByIDAndUserID(ctx, hid, userID)
			}
		}

		if handle == nil && respItem.PlatformID != "" {
			handles, _ := s.socialHandleRepo.FindByUserPlatform(ctx, userID, models.PlatformLinkedIn)
			for _, h := range handles {
				if h.PlatformID == respItem.PlatformID {
					handle = &h
					break
				}
			}
		}

		if handle == nil {
			continue
		}

		platformPostURN := s.linkedinService.Activity.NormalizePostURN(respItem.PlatformPostID)
		if platformPostURN == "" {
			continue
		}

		activity := &models.PostActivity{
			UserID:          userID,
			SocialHandleID:  handle.ID,
			Platform:        "linkedin",
			PlatformPostID:  respItem.PlatformPostID,
			PlatformPostURN: platformPostURN,
			PlatformPostURL: respItem.URL,
			ContentSnapshot: s.buildContentSnapshot(post),
		}

		_, _ = s.postActivityRepo.UpsertLinkedInActivity(ctx, userID, handle.ID, respItem.PlatformPostID, activity)
	}

	return nil
}

func (s *LinkedInActivityService) ListPostLinkedInActivity(ctx context.Context, userID, postID bson.ObjectID) ([]models.PostActivity, error) {
	post, err := s.postRepo.FindByIDAndUserID(ctx, postID, userID)
	if err != nil || post == nil {
		return []models.PostActivity{}, nil
	}

	platformPostIDs := make([]string, 0)
	for _, r := range post.PlatformResponses {
		if strings.ToLower(r.Platform) == "linkedin" && r.Status == "success" {
			platformPostIDs = append(platformPostIDs, strings.TrimSpace(r.PlatformPostID))
		}
	}

	if len(platformPostIDs) == 0 {
		return []models.PostActivity{}, nil
	}

	return s.postActivityRepo.ListByPlatformPostIDs(ctx, userID, platformPostIDs, "linkedin")
}

func (s *LinkedInActivityService) SyncLinkedInHistoricalPostsForAllConnectedAccounts(ctx context.Context, userID bson.ObjectID) (int, int, error) {
	handles, err := s.socialHandleRepo.FindByUserPlatform(ctx, userID, models.PlatformLinkedIn)
	if err != nil || len(handles) == 0 {
		return 0, 0, nil
	}

	savedCount := 0

	for _, handle := range handles {
		token := strings.TrimSpace(handle.PlatformAccessToken)
		if token == "" {
			continue
		}

		historicalPosts, err := s.linkedinService.FetchHistoricalPosts(ctx, token, &handle, 100)
		if err != nil {
			continue
		}

		for _, hp := range historicalPosts {
			urn := strings.TrimSpace(hp.ID)
			platformPostURN := s.linkedinService.Activity.NormalizePostURN(urn)
			platformPostID := s.linkedinService.Activity.PostURNToPlatformPostID(platformPostURN)

			if platformPostURN == "" || platformPostID == "" {
				continue
			}

			var pubTime *time.Time
			if hp.CreatedAt > 0 {
				t := time.UnixMilli(hp.CreatedAt)
				pubTime = &t
			} else if hp.PublishedAt > 0 {
				t := time.UnixMilli(hp.PublishedAt)
				pubTime = &t
			}

			activity := &models.PostActivity{
				UserID:          userID,
				SocialHandleID:  handle.ID,
				Platform:        "linkedin",
				PlatformPostID:  platformPostID,
				PlatformPostURN: platformPostURN,
				PlatformPostURL: "",
				ContentSnapshot: models.ContentSnapshot{
					Title:         "",
					Caption:       s.linkedinService.Activity.ExtractPostText(hp),
					PublishedTime: pubTime,
				},
			}

			_, err = s.postActivityRepo.UpsertLinkedInActivity(ctx, userID, handle.ID, platformPostID, activity)
			if err == nil {
				savedCount++
			}
		}
	}

	return len(handles), savedCount, nil
}
