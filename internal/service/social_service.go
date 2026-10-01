package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type SocialService struct {
	config           *config.Config
	socialHandleRepo *repository.SocialHandleRepository
	instagramService *instagram.Service
	linkedinService  *linkedin.Service
}

func NewSocialService(
	cfg *config.Config,
	repo *repository.SocialHandleRepository,
	igService *instagram.Service,
	liService *linkedin.Service,
) *SocialService {
	return &SocialService{
		config:           cfg,
		socialHandleRepo: repo,
		instagramService: igService,
		linkedinService:  liService,
	}
}

func (s *SocialService) normalizePlatform(p string) string {
	return strings.ToUpper(strings.TrimSpace(p))
}

func (s *SocialService) GetConnectURL(userID, platform string) (string, error) {
	norm := s.normalizePlatform(platform)
	state := common.GenerateOAuthState(userID, s.config.JWTAccessSecret)

	switch norm {
	case models.PlatformInstagram:
		return s.instagramService.GetConnectURL(state), nil
	case models.PlatformLinkedIn:
		return s.linkedinService.GetConnectURL(state), nil
	default:
		return "", utils.BadRequest(fmt.Sprintf("Unsupported platform: %s", platform))
	}
}

func (s *SocialService) ConnectFromCallback(ctx context.Context, userIDStr, platform, code, state string, customRedirectURI ...string) (*models.SocialHandle, error) {
	norm := s.normalizePlatform(platform)
	userOID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return nil, utils.BadRequest("Invalid user ID")
	}

	switch norm {
	case models.PlatformInstagram:
		handle, err := s.instagramService.HandleCallback(ctx, code, state)
		if err != nil {
			return nil, err
		}
		handle.UserID = userOID
		saved, err := s.socialHandleRepo.UpsertByUserPlatformIdentity(ctx, userOID, models.PlatformInstagram, handle.PlatformID, handle)
		if err != nil {
			return nil, err
		}
		return saved, nil

	case models.PlatformLinkedIn:
		primaryHandle, managedPages, err := s.linkedinService.HandleCallback(ctx, code, state, customRedirectURI...)
		if err != nil {
			return nil, err
		}
		primaryHandle.UserID = userOID

		savedPrimary, err := s.socialHandleRepo.UpsertByUserPlatformIdentity(ctx, userOID, models.PlatformLinkedIn, primaryHandle.PlatformID, primaryHandle)
		if err != nil {
			return nil, err
		}

		linkedPagesCount := 0
		for _, page := range managedPages {
			pageHandle := &models.SocialHandle{
				UserID:                     userOID,
				Platform:                   models.PlatformLinkedIn,
				PlatformID:                 page.ID,
				PlatformUsername:           page.Username,
				PlatformDisplayName:        page.DisplayName,
				PlatformAccessToken:        primaryHandle.PlatformAccessToken,
				PlatformRefreshToken:       primaryHandle.PlatformRefreshToken,
				PlatformAccessTokenExpiry:  primaryHandle.PlatformAccessTokenExpiry,
				PlatformRefreshTokenExpiry: primaryHandle.PlatformRefreshTokenExpiry,
				PlatformMetadata: map[string]any{
					"accountType":          models.LinkedInAccountTypeOrganization,
					"parentPlatformUserId": primaryHandle.PlatformID,
					"organizationUrn":      fmt.Sprintf("urn:li:organization:%s", page.ID),
					"profilePicture":       page.ProfilePicture,
				},
			}
			for k, v := range page.PlatformMetadata {
				pageHandle.PlatformMetadata[k] = v
			}

			_, err := s.socialHandleRepo.UpsertByUserPlatformIdentity(ctx, userOID, models.PlatformLinkedIn, page.ID, pageHandle)
			if err == nil {
				linkedPagesCount++
			}
		}

		savedPrimary.LinkedPageCount = linkedPagesCount
		if linkedPagesCount == 0 {
			savedPrimary.Warning = "No LinkedIn company pages were found. Reconnect LinkedIn with organization scopes."
		}
		return savedPrimary, nil

	default:
		return nil, utils.BadRequest("Unsupported platform")
	}
}

func (s *SocialService) DisconnectAccount(ctx context.Context, userID, socialID bson.ObjectID) error {
	deleted, err := s.socialHandleRepo.DeleteByIDAndUserID(ctx, socialID, userID)
	if err != nil {
		return err
	}
	if deleted == nil {
		return utils.NotFound("Social account not found")
	}
	return nil
}

func (s *SocialService) RefreshPlatformToken(ctx context.Context, userID, socialID bson.ObjectID) (*models.SocialHandle, error) {
	social, err := s.socialHandleRepo.FindByIDAndUserID(ctx, socialID, userID)
	if err != nil || social == nil {
		return nil, utils.NotFound("Social account not found")
	}

	switch social.Platform {
	case models.PlatformInstagram:
		refreshCred := social.PlatformRefreshToken
		if refreshCred == "" {
			refreshCred = social.PlatformAccessToken
		}
		if refreshCred == "" {
			return nil, utils.BadRequest("No refresh token available for Instagram account")
		}

		refreshed, err := s.instagramService.RefreshToken(ctx, refreshCred)
		if err != nil || refreshed == nil || refreshed.AccessToken == "" {
			_, _ = s.socialHandleRepo.DeleteByIDAndUserID(ctx, social.ID, userID)
			return nil, utils.BadRequest("Failed to refresh Instagram token")
		}

		expiresIn := refreshed.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 60 * 24 * 3600
		}
		expiryDate := time.Now().Add(time.Duration(expiresIn) * time.Second)

		return s.socialHandleRepo.UpdateByIDAndUserID(ctx, social.ID, userID, bson.M{
			"platformAccessToken":        refreshed.AccessToken,
			"platformRefreshToken":       refreshed.AccessToken,
			"platformAccessTokenExpiry":  expiryDate,
			"platformRefreshTokenExpiry": expiryDate,
		})

	case models.PlatformLinkedIn:
		if social.PlatformRefreshToken == "" {
			return nil, utils.BadRequest("No refresh token available for LinkedIn account")
		}

		refreshed, err := s.linkedinService.RefreshToken(ctx, social.PlatformRefreshToken)
		if err != nil || refreshed == nil || refreshed.AccessToken == "" {
			_, _ = s.socialHandleRepo.DeleteByIDAndUserID(ctx, social.ID, userID)
			return nil, utils.BadRequest("Failed to refresh LinkedIn token")
		}

		expiresIn := refreshed.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 60 * 24 * 3600
		}
		expiryDate := time.Now().Add(time.Duration(expiresIn) * time.Second)

		update := bson.M{
			"platformAccessToken":       refreshed.AccessToken,
			"platformAccessTokenExpiry": expiryDate,
		}
		if refreshed.RefreshToken != "" {
			update["platformRefreshToken"] = refreshed.RefreshToken
		}

		return s.socialHandleRepo.UpdateByIDAndUserID(ctx, social.ID, userID, update)

	default:
		return nil, utils.BadRequest("Unsupported platform for token refresh")
	}
}

func (s *SocialService) RefreshPlatformTokenIfNeeded(ctx context.Context, userID bson.ObjectID, handle *models.SocialHandle) *models.SocialHandle {
	if handle == nil || handle.ID.IsZero() {
		return handle
	}

	if handle.Platform == models.PlatformInstagram {
		if handle.PlatformAccessTokenExpiry == nil {
			return handle
		}
		// Refresh window: 10 minutes
		if time.Until(*handle.PlatformAccessTokenExpiry) <= 10*time.Minute {
			refreshed, err := s.RefreshPlatformToken(ctx, userID, handle.ID)
			if err == nil && refreshed != nil {
				return refreshed
			}
		}
		return handle
	}

	if handle.PlatformAccessTokenExpiry != nil && time.Now().After(*handle.PlatformAccessTokenExpiry) {
		refreshed, err := s.RefreshPlatformToken(ctx, userID, handle.ID)
		if err == nil && refreshed != nil {
			return refreshed
		}
	}

	return handle
}

func (s *SocialService) SyncLinkedInPages(ctx context.Context, userID bson.ObjectID) {
	accounts, err := s.socialHandleRepo.FindByUserPlatform(ctx, userID, models.PlatformLinkedIn)
	if err != nil || len(accounts) == 0 {
		return
	}

	for _, handle := range accounts {
		accountType := ""
		isChildPage := false
		if handle.PlatformMetadata != nil {
			if at, ok := handle.PlatformMetadata["accountType"].(string); ok {
				accountType = at
			}
			if parent, ok := handle.PlatformMetadata["parentPlatformUserId"].(string); ok && parent != "" {
				isChildPage = true
			}
		}

		if isChildPage || models.IsLinkedInOrganization(accountType) {
			continue
		}

		pages, err := s.linkedinService.OAuth.FetchManagedPages(ctx, handle.PlatformAccessToken, nil)
		if err != nil {
			continue
		}

		for _, page := range pages {
			pageHandle := &models.SocialHandle{
				UserID:                     userID,
				Platform:                   models.PlatformLinkedIn,
				PlatformID:                 page.ID,
				PlatformUsername:           page.Username,
				PlatformDisplayName:        page.DisplayName,
				PlatformAccessToken:        handle.PlatformAccessToken,
				PlatformRefreshToken:       handle.PlatformRefreshToken,
				PlatformAccessTokenExpiry:  handle.PlatformAccessTokenExpiry,
				PlatformRefreshTokenExpiry: handle.PlatformRefreshTokenExpiry,
				PlatformMetadata: map[string]any{
					"accountType":          models.LinkedInAccountTypeOrganization,
					"parentPlatformUserId": handle.PlatformID,
					"organizationUrn":      fmt.Sprintf("urn:li:organization:%s", page.ID),
					"profilePicture":       page.ProfilePicture,
				},
			}
			for k, v := range page.PlatformMetadata {
				pageHandle.PlatformMetadata[k] = v
			}

			_, _ = s.socialHandleRepo.UpsertByUserPlatformIdentity(ctx, userID, models.PlatformLinkedIn, page.ID, pageHandle)
		}
	}
}

func (s *SocialService) ListAccounts(ctx context.Context, userID bson.ObjectID, platformFilter string, skip, limit int, sort bson.D) ([]models.SocialHandle, int64, error) {
	norm := s.normalizePlatform(platformFilter)
	filters := bson.M{}
	if norm != "" {
		filters["platform"] = norm
	}

	// Fire-and-forget LinkedIn pages sync to keep accounts list responsive
	if norm == "" || norm == models.PlatformLinkedIn {
		go s.SyncLinkedInPages(context.Background(), userID)
	}

	items, err := s.socialHandleRepo.ListByUserID(ctx, userID, filters, skip, limit, sort)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.socialHandleRepo.CountByUserID(ctx, userID, filters)
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (s *SocialService) ListHandlesForPlatforms(ctx context.Context, userID bson.ObjectID, platforms []string) ([]models.SocialHandle, error) {
	return s.socialHandleRepo.FindByUserPlatforms(ctx, userID, platforms)
}

func (s *SocialService) CountConnected(ctx context.Context, userID bson.ObjectID, accountIDs []string) (int64, error) {
	filters := bson.M{}
	if len(accountIDs) > 0 {
		oids := make([]bson.ObjectID, 0, len(accountIDs))
		for _, id := range accountIDs {
			if oid, err := bson.ObjectIDFromHex(id); err == nil {
				oids = append(oids, oid)
			}
		}
		filters["_id"] = bson.M{"$in": oids}
	}
	return s.socialHandleRepo.CountByUserID(ctx, userID, filters)
}
