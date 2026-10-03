package service

import (
	"context"

	"encoding/json"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/database"
	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type UserService struct {
	config           *config.Config
	userRepo         *repository.UserRepository
	refreshTokenRepo *repository.RefreshTokenRepository
	redis            *database.RedisClient
}

func NewUserService(cfg *config.Config, userRepo *repository.UserRepository, rfRepo *repository.RefreshTokenRepository, redis *database.RedisClient) *UserService {
	return &UserService{
		config:           cfg,
		userRepo:         userRepo,
		refreshTokenRepo: rfRepo,
		redis:            redis,
	}
}

func (s *UserService) GetUserByID(ctx context.Context, userID bson.ObjectID) (*models.User, error) {
	cacheKey := "user:" + userID.Hex()
	if s.redis != nil {
		if cachedJSON, err := s.redis.Get(ctx, cacheKey); err == nil && cachedJSON != "" {
			var cached models.User
			if json.Unmarshal([]byte(cachedJSON), &cached) == nil {
				return &cached, nil
			}
		}
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || user == nil {
		return user, err
	}

	if s.redis != nil {
		if b, err := json.Marshal(user); err == nil {
			_ = s.redis.Set(ctx, cacheKey, string(b), 10*time.Minute)
		}
	}

	return user, nil
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

	if s.redis != nil {
		_ = s.redis.Del(ctx, "user:"+userID.Hex())
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

	if s.redis != nil {
		_ = s.redis.Del(ctx, "user:"+userID.Hex())
	}

	_ = s.refreshTokenRepo.RevokeAllByUserID(ctx, userID)
	return true, nil
}

func (s *UserService) DeleteUser(ctx context.Context, userID bson.ObjectID) error {
	if s.redis != nil {
		_ = s.redis.Del(ctx, "user:"+userID.Hex())
	}
	_ = s.refreshTokenRepo.RevokeAllByUserID(ctx, userID)
	return s.userRepo.DeleteByID(ctx, userID)
}
