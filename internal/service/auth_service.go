package service

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type AuthTokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type LoginResult struct {
	User   models.UserSanitized `json:"user"`
	Tokens AuthTokens           `json:"tokens"`
}

type AuthService struct {
	config       *config.Config
	userRepo     *repository.UserRepository
	tokenService *TokenService
}

func NewAuthService(cfg *config.Config, userRepo *repository.UserRepository, tokenService *TokenService) *AuthService {
	return &AuthService{
		config:       cfg,
		userRepo:     userRepo,
		tokenService: tokenService,
	}
}

func (s *AuthService) Signup(ctx context.Context, email, password string) (*models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	existing, err := s.userRepo.FindByEmail(ctx, normEmail)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, utils.Conflict("Email is already registered")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), s.config.BcryptSaltRounds)
	if err != nil {
		return nil, utils.InternalServerError("Failed to hash password")
	}

	verificationToken := utils.RandomToken(32)
	verificationExpiry := time.Now().Add(15 * time.Minute)

	user := &models.User{
		Email:                      normEmail,
		Password:                   string(hashedPassword),
		FullName:                   "New User",
		IsVerified:                 false,
		EmailVerificationToken:     &verificationToken,
		EmailVerificationExpiresAt: &verificationExpiry,
		Timezone:                   "UTC",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	user, err := s.userRepo.FindByVerificationToken(ctx, token)
	if err != nil || user == nil {
		return utils.BadRequest("Invalid or expired verification token")
	}

	if user.EmailVerificationExpiresAt == nil || user.EmailVerificationExpiresAt.Before(time.Now()) {
		return utils.BadRequest("Invalid or expired verification token")
	}

	_, err = s.userRepo.UpdateByID(ctx, user.ID, bson.M{
		"isVerified":                 true,
		"emailVerificationToken":     nil,
		"emailVerificationExpiresAt": nil,
	})
	return err
}

func (s *AuthService) Login(ctx context.Context, email, password, userAgent, ip string) (*LoginResult, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	user, err := s.userRepo.FindByEmail(ctx, normEmail)
	if err != nil || user == nil {
		return nil, utils.Unauthorized("Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, utils.Unauthorized("Invalid credentials")
	}

	if !user.IsVerified {
		return nil, utils.Forbidden("Please verify your email first")
	}

	accessToken, err := s.tokenService.GenerateAccessToken(user.ID.Hex())
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.tokenService.GenerateRefreshToken(ctx, user.ID, userAgent, ip)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	_, _ = s.userRepo.UpdateByID(ctx, user.ID, bson.M{
		"lastLoginAt": now,
	})

	return &LoginResult{
		User: user.ToSanitized(),
		Tokens: AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken, userAgent, ip string) (*LoginResult, error) {
	rotated, err := s.tokenService.RotateRefreshToken(ctx, rawRefreshToken, userAgent, ip)
	if err != nil {
		return nil, err
	}

	uid, err := bson.ObjectIDFromHex(rotated.UserID)
	if err != nil {
		return nil, utils.Unauthorized("Invalid refresh token user")
	}

	user, err := s.userRepo.FindByID(ctx, uid)
	if err != nil || user == nil {
		return nil, utils.Unauthorized("Invalid refresh token user")
	}

	return &LoginResult{
		User: user.ToSanitized(),
		Tokens: AuthTokens{
			AccessToken:  rotated.AccessToken,
			RefreshToken: rotated.RefreshToken,
		},
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	if rawRefreshToken == "" {
		return nil
	}
	_, err := s.tokenService.RevokeRefreshToken(ctx, rawRefreshToken)
	return err
}

func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	user, err := s.userRepo.FindByEmail(ctx, normEmail)
	if err != nil || user == nil {
		return nil // Don't leak user existence
	}

	resetToken := utils.RandomToken(32)
	resetExpiry := time.Now().Add(1 * time.Hour)

	_, err = s.userRepo.UpdateByID(ctx, user.ID, bson.M{
		"resetPasswordToken":     resetToken,
		"resetPasswordExpiresAt": resetExpiry,
	})
	return err
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	user, err := s.userRepo.FindByResetToken(ctx, token)
	if err != nil || user == nil {
		return utils.BadRequest("Invalid or expired reset token")
	}

	if user.ResetPasswordExpiresAt == nil || user.ResetPasswordExpiresAt.Before(time.Now()) {
		return utils.BadRequest("Invalid or expired reset token")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.config.BcryptSaltRounds)
	if err != nil {
		return utils.InternalServerError("Failed to hash password")
	}

	_ = s.tokenService.RevokeAllForUser(ctx, user.ID)

	_, err = s.userRepo.UpdateByID(ctx, user.ID, bson.M{
		"password":               string(hashedPassword),
		"resetPasswordToken":     nil,
		"resetPasswordExpiresAt": nil,
	})
	return err
}
