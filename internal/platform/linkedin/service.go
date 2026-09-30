package linkedin

import (
	"context"
	"strings"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
)

type Service struct {
	OAuth    *OAuthAdapter
	Publish  *PublishAdapter
	Activity *ActivityAdapter
}

func NewService(cfg config.OAuthPlatformConfig) *Service {
	return &Service{
		OAuth:    NewOAuthAdapter(cfg),
		Publish:  NewPublishAdapter(cfg),
		Activity: NewActivityAdapter(cfg),
	}
}

func (s *Service) GetConnectURL(state string) string {
	return s.OAuth.BuildAuthorizeURL(state)
}

func (s *Service) HandleCallback(ctx context.Context, code, state string) (*models.SocialHandle, []ManagedPage, error) {
	tokenResp, err := s.OAuth.ExchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, nil, err
	}

	accessToken := tokenResp.AccessToken
	refreshToken := tokenResp.RefreshToken
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 60 * 24 * 3600 // 60 days
	}
	expiryDate := time.Now().Add(time.Duration(expiresIn) * time.Second)

	profile, err := s.OAuth.FetchUserProfile(ctx, accessToken)
	if err != nil {
		return nil, nil, err
	}

	platformID := profile.Sub
	if platformID == "" {
		platformID = profile.ID
	}

	username := profile.Email
	if username == "" {
		username = profile.Name
	}
	if username == "" {
		username = platformID
	}

	displayName := profile.Name
	if displayName == "" {
		displayName = username
	}

	scopesList := strings.Fields(tokenResp.Scope)

	primaryHandle := &models.SocialHandle{
		Platform:                   models.PlatformLinkedIn,
		PlatformID:                 platformID,
		PlatformUsername:           username,
		PlatformDisplayName:        displayName,
		PlatformAccessToken:        accessToken,
		PlatformRefreshToken:       refreshToken,
		PlatformAccessTokenExpiry:  &expiryDate,
		PlatformRefreshTokenExpiry: &expiryDate,
		PlatformMetadata: map[string]any{
			"accountType":    models.LinkedInAccountTypePerson,
			"subjectId":      profile.Sub,
			"email":          profile.Email,
			"emailVerified":  profile.EmailVerified,
			"profilePicture": profile.Picture,
			"scopes":         scopesList,
		},
	}

	managedPages, _ := s.OAuth.FetchManagedPages(ctx, accessToken, profile)

	return primaryHandle, managedPages, nil
}

func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	return s.OAuth.RefreshToken(ctx, refreshToken)
}

func (s *Service) PublishPost(ctx context.Context, post *models.Post, handle *models.SocialHandle) (*models.PlatformPublishResponse, error) {
	res, err := s.Publish.Publish(ctx, post, handle)
	if err != nil {
		return nil, err
	}

	// Check if token was revoked
	if res.Status == "failed" {
		msg := strings.ToLower(res.Message)
		if strings.Contains(msg, "revoked") || strings.Contains(msg, "token") {
			// Mark that access was revoked
			res.Message = "LinkedIn access was revoked. Reconnect LinkedIn from Accounts, then publish again."
		}
	}
	return res, nil
}

func (s *Service) DeletePost(ctx context.Context, accessToken, platformPostID string) error {
	return s.Publish.DeletePost(ctx, accessToken, platformPostID)
}

func (s *Service) FetchHistoricalPosts(ctx context.Context, accessToken string, handle *models.SocialHandle, maxPosts int) ([]HistoricalPost, error) {
	return s.Activity.FetchAuthorHistoricalPosts(ctx, accessToken, handle, maxPosts)
}
