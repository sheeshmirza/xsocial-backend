package instagram

import (
	"context"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
)

type Service struct {
	OAuth   *OAuthAdapter
	Publish *PublishAdapter
}

func NewService(cfg config.OAuthPlatformConfig) *Service {
	return &Service{
		OAuth:   NewOAuthAdapter(cfg),
		Publish: NewPublishAdapter(),
	}
}

func (s *Service) GetConnectURL(state string) string {
	return s.OAuth.BuildAuthorizeURL(state, nil)
}

func (s *Service) HandleCallback(ctx context.Context, code, state string) (*models.SocialHandle, error) {
	tokenResp, err := s.OAuth.ExchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, err
	}

	accessToken := tokenResp.AccessToken
	refreshToken := tokenResp.AccessToken
	expiresIn := tokenResp.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}

	// Upgrade short-lived token (1h) to long-lived token (60 days)
	longLived, err := s.OAuth.ExchangeForLongLivedToken(ctx, accessToken)
	if err == nil && longLived.AccessToken != "" {
		accessToken = longLived.AccessToken
		refreshToken = longLived.AccessToken
		if longLived.ExpiresIn > 0 {
			expiresIn = longLived.ExpiresIn
		}
	}

	expiryDate := time.Now().Add(time.Duration(expiresIn) * time.Second)

	profile, err := s.OAuth.FetchUserProfile(ctx, accessToken, "")
	if err != nil {
		return nil, err
	}

	accountType := models.NormalizeAccountTypeByPlatform("instagram", profile.AccountType)

	handle := &models.SocialHandle{
		Platform:                   models.PlatformInstagram,
		PlatformID:                 profile.ID,
		PlatformUsername:           profile.Username,
		PlatformDisplayName:        profile.Name,
		PlatformAccessToken:        accessToken,
		PlatformRefreshToken:       refreshToken,
		PlatformAccessTokenExpiry:  &expiryDate,
		PlatformRefreshTokenExpiry: &expiryDate,
		PlatformMetadata: map[string]any{
			"accountType":    accountType,
			"profilePicture": profile.ProfilePictureURL,
		},
	}

	return handle, nil
}

func (s *Service) RefreshToken(ctx context.Context, token string) (*TokenResponse, error) {
	return s.OAuth.RefreshToken(ctx, token)
}

func (s *Service) PublishPost(ctx context.Context, post *models.Post, handle *models.SocialHandle) (*models.PlatformPublishResponse, error) {
	return s.Publish.Publish(ctx, post, handle)
}
