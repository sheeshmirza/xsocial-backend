package service

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type CompetitorService struct {
	repo      *repository.CompetitorRepository
	brandRepo *repository.BrandRepository
}

func NewCompetitorService(repo *repository.CompetitorRepository, brandRepo *repository.BrandRepository) *CompetitorService {
	return &CompetitorService{
		repo:      repo,
		brandRepo: brandRepo,
	}
}

func (s *CompetitorService) AddCompetitor(ctx context.Context, brandID bson.ObjectID, req models.AddCompetitorRequest) (*models.Competitor, error) {
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform == "" {
		return nil, utils.BadRequest("platform is required (e.g. instagram, facebook, youtube, twitter, twitch)")
	}

	handle := strings.TrimSpace(req.Handle)
	if handle == "" {
		return nil, utils.BadRequest("competitor handle is required")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = handle
	}

	today := time.Now().Format("2006-01-02")
	var history []models.CompetitorSnapshot
	if req.Followers > 0 || req.PostsCount > 0 || req.EngagementRate > 0 {
		history = append(history, models.CompetitorSnapshot{
			Date:           today,
			Followers:      req.Followers,
			PostsCount:     req.PostsCount,
			EngagementRate: req.EngagementRate,
		})
	} else {
		history = []models.CompetitorSnapshot{}
	}

	comp := &models.Competitor{
		BrandID:        brandID,
		Platform:       platform,
		Handle:         handle,
		Name:           name,
		AvatarURL:      req.AvatarURL,
		ExternalID:     req.ExternalID,
		Followers:      req.Followers,
		Following:      req.Following,
		PostsCount:     req.PostsCount,
		EngagementRate: req.EngagementRate,
		History:        history,
	}

	if err := s.repo.Create(ctx, comp); err != nil {
		return nil, utils.InternalServerError("Failed to track competitor")
	}
	return comp, nil
}

func (s *CompetitorService) ListCompetitors(ctx context.Context, brandID bson.ObjectID) ([]models.Competitor, error) {
	return s.repo.FindByBrand(ctx, brandID)
}

func (s *CompetitorService) DeleteCompetitor(ctx context.Context, id, brandID bson.ObjectID) error {
	deleted, err := s.repo.Delete(ctx, id, brandID)
	if err != nil {
		return utils.InternalServerError("Failed to delete competitor")
	}
	if !deleted {
		return utils.NotFound("Competitor not found")
	}
	return nil
}

func (s *CompetitorService) RecordSnapshot(ctx context.Context, id, brandID bson.ObjectID, snap models.CompetitorSnapshot) error {
	comp, err := s.repo.FindByID(ctx, id)
	if err != nil || comp == nil || comp.BrandID != brandID {
		return utils.NotFound("Competitor not found")
	}

	if snap.Date == "" {
		snap.Date = time.Now().Format("2006-01-02")
	}

	return s.repo.AddSnapshot(ctx, id, snap)
}

func (s *CompetitorService) SyncCompetitorMetrics(ctx context.Context, id, brandID bson.ObjectID, manualSnapshot *models.CompetitorSnapshot) error {
	comp, err := s.repo.FindByID(ctx, id)
	if err != nil || comp == nil || comp.BrandID != brandID {
		return utils.NotFound("Competitor not found")
	}

	if manualSnapshot != nil {
		return s.RecordSnapshot(ctx, id, brandID, *manualSnapshot)
	}

	// Update lastSyncedAt timestamp when no external provider payload was given
	return s.repo.Update(ctx, id, bson.M{
		"lastSyncedAt": time.Now(),
	})
}
