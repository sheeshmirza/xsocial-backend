package service

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type RotateResult struct {
	UserID       string
	AccessToken  string
	RefreshToken string
}

type TokenService struct {
	config           *config.Config
	refreshTokenRepo repository.IRefreshTokenRepository
}

func NewTokenService(cfg *config.Config, repo repository.IRefreshTokenRepository) *TokenService {
	return &TokenService{
		config:           cfg,
		refreshTokenRepo: repo,
	}
}

func (s *TokenService) GenerateAccessToken(userID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": now.Unix(),
		"exp": now.Add(s.config.JWTAccessExpiresIn).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTAccessSecret))
}

func (s *TokenService) GenerateRefreshToken(ctx context.Context, userID bson.ObjectID, userAgent, ip string) (string, error) {
	rawToken := utils.RandomToken(48)
	tokenHash := utils.SHA256(rawToken)
	tokenExpiresAt := time.Now().Add(s.config.JWTRefreshExpiresIn)

	rf := &models.RefreshToken{
		UserID:         userID,
		TokenHash:      tokenHash,
		TokenExpiresAt: tokenExpiresAt,
		UserAgent:      userAgent,
		UserIPAddress:  ip,
	}

	if err := s.refreshTokenRepo.Create(ctx, rf); err != nil {
		return "", err
	}
	return rawToken, nil
}

func (s *TokenService) RotateRefreshToken(ctx context.Context, rawToken, userAgent, ip string) (*RotateResult, error) {
	tokenHash := utils.SHA256(rawToken)
	tokenDoc, err := s.refreshTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil || tokenDoc == nil {
		return nil, utils.Unauthorized("Invalid refresh token")
	}

	if tokenDoc.TokenRevokedAt != nil || tokenDoc.TokenExpiresAt.Before(time.Now()) {
		return nil, utils.Unauthorized("Invalid or expired refresh token")
	}

	_ = s.refreshTokenRepo.RevokeByID(ctx, tokenDoc.ID)

	newRefreshToken, err := s.GenerateRefreshToken(ctx, tokenDoc.UserID, userAgent, ip)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.GenerateAccessToken(tokenDoc.UserID.Hex())
	if err != nil {
		return nil, err
	}

	return &RotateResult{
		UserID:       tokenDoc.UserID.Hex(),
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *TokenService) RevokeRefreshToken(ctx context.Context, rawToken string) (bool, error) {
	tokenHash := utils.SHA256(rawToken)
	tokenDoc, err := s.refreshTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil || tokenDoc == nil {
		return false, nil
	}
	if tokenDoc.TokenRevokedAt != nil {
		return false, nil
	}
	err = s.refreshTokenRepo.RevokeByID(ctx, tokenDoc.ID)
	return err == nil, err
}

func (s *TokenService) RevokeAllForUser(ctx context.Context, userID bson.ObjectID) error {
	return s.refreshTokenRepo.RevokeAllByUserID(ctx, userID)
}
