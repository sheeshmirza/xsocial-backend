package service

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type BrandService struct {
	repo     *repository.BrandRepository
	userRepo *repository.UserRepository
}

func NewBrandService(repo *repository.BrandRepository, userRepo *repository.UserRepository) *BrandService {
	return &BrandService{
		repo:     repo,
		userRepo: userRepo,
	}
}

func (s *BrandService) CreateBrand(ctx context.Context, userID bson.ObjectID, req models.CreateBrandRequest) (*models.Brand, error) {
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	curr := req.Currency
	if curr == "" {
		curr = "USD"
	}

	brand := &models.Brand{
		OwnerID:     userID,
		Name:        req.Name,
		Description: req.Description,
		LogoURL:     req.LogoURL,
		Timezone:    tz,
		Currency:    curr,
		WebsiteURL:  req.WebsiteURL,
	}

	if err := s.repo.CreateBrand(ctx, brand); err != nil {
		return nil, utils.InternalServerError("Failed to create brand")
	}
	return brand, nil
}

func (s *BrandService) GetBrand(ctx context.Context, id, userID bson.ObjectID) (*models.Brand, error) {
	// Verify membership
	member, err := s.repo.FindMember(ctx, id, userID)
	if err != nil || member == nil {
		return nil, utils.Forbidden("Access to brand denied")
	}

	brand, err := s.repo.FindBrandByID(ctx, id)
	if err != nil {
		return nil, utils.InternalServerError("Failed to retrieve brand")
	}
	if brand == nil {
		return nil, utils.NotFound("Brand not found")
	}
	return brand, nil
}

func (s *BrandService) ListBrands(ctx context.Context, userID bson.ObjectID) ([]models.Brand, error) {
	brands, err := s.repo.FindBrandsByUserID(ctx, userID)
	if err != nil {
		return nil, utils.InternalServerError("Failed to list brands")
	}
	return brands, nil
}

func (s *BrandService) UpdateBrand(ctx context.Context, id, userID bson.ObjectID, req models.UpdateBrandRequest) (*models.Brand, error) {
	// Verify admin or owner role
	member, err := s.repo.FindMember(ctx, id, userID)
	if err != nil || member == nil || (member.Role != models.BrandRoleOwner && member.Role != models.BrandRoleAdmin) {
		return nil, utils.Forbidden("Only brand owner or admin can modify brand settings")
	}

	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.LogoURL != nil {
		update["logoUrl"] = *req.LogoURL
	}
	if req.Timezone != nil {
		update["timezone"] = *req.Timezone
	}
	if req.Currency != nil {
		update["currency"] = *req.Currency
	}
	if req.WebsiteURL != nil {
		update["websiteUrl"] = *req.WebsiteURL
	}

	if len(update) == 0 {
		return s.repo.FindBrandByID(ctx, id)
	}

	updated, err := s.repo.UpdateBrand(ctx, id, update)
	if err != nil {
		return nil, utils.InternalServerError("Failed to update brand")
	}
	return updated, nil
}

func (s *BrandService) DeleteBrand(ctx context.Context, id, userID bson.ObjectID) error {
	// Only owner can delete brand
	brand, err := s.repo.FindBrandByID(ctx, id)
	if err != nil || brand == nil {
		return utils.NotFound("Brand not found")
	}
	if brand.OwnerID != userID {
		return utils.Forbidden("Only brand owner can delete this brand")
	}

	deleted, err := s.repo.DeleteBrand(ctx, id)
	if err != nil {
		return utils.InternalServerError("Failed to delete brand")
	}
	if !deleted {
		return utils.NotFound("Brand not found")
	}
	return nil
}

func (s *BrandService) AddMember(ctx context.Context, brandID, userID bson.ObjectID, req models.AddBrandMemberRequest) (*models.BrandMember, error) {
	// Verify caller is admin or owner
	caller, err := s.repo.FindMember(ctx, brandID, userID)
	if err != nil || caller == nil || (caller.Role != models.BrandRoleOwner && caller.Role != models.BrandRoleAdmin) {
		return nil, utils.Forbidden("Only owner or admin can invite members")
	}

	targetUser, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil || targetUser == nil {
		return nil, utils.NotFound("User with this email not found")
	}

	member := &models.BrandMember{
		BrandID: brandID,
		UserID:  targetUser.ID,
		Email:   targetUser.Email,
		Name:    targetUser.FullName,
		Role:    req.Role,
	}

	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, utils.Conflict("User is already a member of this brand")
	}
	return member, nil
}

func (s *BrandService) ListMembers(ctx context.Context, brandID, userID bson.ObjectID) ([]models.BrandMember, error) {
	caller, err := s.repo.FindMember(ctx, brandID, userID)
	if err != nil || caller == nil {
		return nil, utils.Forbidden("Access denied")
	}
	return s.repo.FindMembersByBrandID(ctx, brandID)
}

func (s *BrandService) RemoveMember(ctx context.Context, brandID, targetUserID, callerUserID bson.ObjectID) error {
	caller, err := s.repo.FindMember(ctx, brandID, callerUserID)
	if err != nil || caller == nil || (caller.Role != models.BrandRoleOwner && caller.Role != models.BrandRoleAdmin) {
		return utils.Forbidden("Only owner or admin can remove members")
	}

	brand, err := s.repo.FindBrandByID(ctx, brandID)
	if err == nil && brand != nil && brand.OwnerID == targetUserID {
		return utils.BadRequest("Cannot remove the brand owner")
	}

	removed, err := s.repo.RemoveMember(ctx, brandID, targetUserID)
	if err != nil {
		return utils.InternalServerError("Failed to remove member")
	}
	if !removed {
		return utils.NotFound("Member not found in this brand")
	}
	return nil
}
