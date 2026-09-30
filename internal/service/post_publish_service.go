package service

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type PostPublishService struct {
	socialHandleRepo *repository.SocialHandleRepository
	socialService    *SocialService
	instagramService *instagram.Service
	linkedinService  *linkedin.Service
}

func NewPostPublishService(
	repo *repository.SocialHandleRepository,
	socialService *SocialService,
	igService *instagram.Service,
	liService *linkedin.Service,
) *PostPublishService {
	return &PostPublishService{
		socialHandleRepo: repo,
		socialService:    socialService,
		instagramService: igService,
		linkedinService:  liService,
	}
}

func (s *PostPublishService) resolveHandles(ctx context.Context, userID bson.ObjectID, platforms, accountIDs []string) ([]models.SocialHandle, error) {
	if len(accountIDs) > 0 {
		return s.socialHandleRepo.FindByUserAndIDs(ctx, userID, accountIDs)
	}
	return s.socialHandleRepo.FindByUserPlatforms(ctx, userID, platforms)
}

func (s *PostPublishService) PublishToPlatforms(
	ctx context.Context,
	userID bson.ObjectID,
	post *models.Post,
	selectedPlatforms, selectedAccountIDs []string,
) ([]models.PlatformPublishResponse, error) {
	if len(selectedPlatforms) == 0 && len(selectedAccountIDs) == 0 {
		return nil, utils.BadRequest("No account or platform selected for publishing")
	}

	handles, err := s.resolveHandles(ctx, userID, selectedPlatforms, selectedAccountIDs)
	if err != nil {
		return nil, err
	}
	if len(handles) == 0 {
		return nil, utils.BadRequest("No connected social handles available for selected accounts/platforms")
	}

	responses := make([]models.PlatformPublishResponse, 0, len(handles))

	for _, handle := range handles {
		freshHandle := s.socialService.RefreshPlatformTokenIfNeeded(ctx, userID, &handle)
		platformNorm := strings.ToUpper(strings.TrimSpace(freshHandle.Platform))

		var res *models.PlatformPublishResponse
		var pubErr error

		switch platformNorm {
		case models.PlatformInstagram:
			res, pubErr = s.instagramService.PublishPost(ctx, post, freshHandle)

		case models.PlatformLinkedIn:
			res, pubErr = s.linkedinService.PublishPost(ctx, post, freshHandle)
			if res != nil && strings.Contains(strings.ToLower(res.Message), "revoked") {
				// Delete revoked handle
				_ = s.socialHandleRepo.DeleteByIDAndUserID(ctx, freshHandle.ID, userID)
			}

		default:
			res = &models.PlatformPublishResponse{
				Platform: strings.ToLower(freshHandle.Platform),
				Status:   "failed",
				Message:  "Publish adapter not available for platform",
			}
		}

		if pubErr != nil {
			responses = append(responses, models.PlatformPublishResponse{
				Platform:       strings.ToLower(freshHandle.Platform),
				Status:         "failed",
				Message:        pubErr.Error(),
				SocialHandleID: freshHandle.ID.Hex(),
				PlatformID:     freshHandle.PlatformID,
			})
			continue
		}

		if res != nil {
			res.SocialHandleID = freshHandle.ID.Hex()
			res.PlatformID = freshHandle.PlatformID
			if freshHandle.PlatformMetadata != nil {
				if at, ok := freshHandle.PlatformMetadata["accountType"].(string); ok {
					res.AccountType = at
				}
			}
			responses = append(responses, *res)
		}
	}

	return responses, nil
}
