package service

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type UserService struct {
	config           *config.Config
	userRepo         *repository.UserRepository
	refreshTokenRepo *repository.RefreshTokenRepository
}

func NewUserService(cfg *config.Config, userRepo *repository.UserRepository, rfRepo *repository.RefreshTokenRepository) *UserService {
	return &UserService{
		config:           cfg,
		userRepo:         userRepo,
		refreshTokenRepo: rfRepo,
	}
}

func (s *UserService) GetUserByID(ctx context.Context, userID bson.ObjectID) (*models.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

func (s *UserService) UpdateProfile(ctx context.Context, userID bson.ObjectID, fullName, timezone string) (*models.User, error) {
	update := bson.M{}
	if fullName != "" {
		update["fullName"] = fullName
	}
	if timezone != "" {
		update["timezone"] = timezone
	}

	user, err := s.userRepo.UpdateByID(ctx, userID, update)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, utils.NotFound("User not found")
	}
	return user, nil
}

func (s *UserService) ChangePassword(ctx context.Context, userID bson.ObjectID, oldPassword, newPassword string) (bool, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return false, utils.NotFound("User not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return false, utils.BadRequest("Old password is incorrect")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.config.BcryptSaltRounds)
	if err != nil {
		return false, utils.InternalServerError("Failed to hash new password")
	}

	_, err = s.userRepo.UpdateByID(ctx, userID, bson.M{
		"password": string(hashedPassword),
	})
	if err != nil {
		return false, err
	}

	_ = s.refreshTokenRepo.RevokeAllByUserID(ctx, userID)
	return true, nil
}

func (s *UserService) DeleteUser(ctx context.Context, userID bson.ObjectID) error {
	_ = s.refreshTokenRepo.RevokeAllByUserID(ctx, userID)
	return s.userRepo.DeleteByID(ctx, userID)
}
