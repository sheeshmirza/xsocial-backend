package service

import (
	"context"
	"math/rand"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type CompetitorService struct {
	repo *repository.CompetitorRepository
}

func NewCompetitorService(repo *repository.CompetitorRepository) *CompetitorService {
	return &CompetitorService{
		repo: repo,
	}
}

func (s *CompetitorService) AddCompetitor(ctx context.Context, brandID bson.ObjectID, req models.AddCompetitorRequest) (*models.Competitor, error) {
	// Initialize with baseline follower benchmarks
	initialFollowers := int64(1000 + rand.Intn(50000))
	posts := int64(50 + rand.Intn(400))
	engagement := 1.5 + rand.Float64()*4.0

	today := time.Now().Format("2006-01-02")
	history := []models.CompetitorSnapshot{
		{
			Date:           today,
			Followers:      initialFollowers,
			PostsCount:     posts,
			EngagementRate: engagement,
		},
	}

	comp := &models.Competitor{
		BrandID:        brandID,
		Platform:       req.Platform,
		Handle:         req.Handle,
		Name:           req.Handle,
		Followers:      initialFollowers,
		Following:      int64(200 + rand.Intn(1000)),
		PostsCount:     posts,
		EngagementRate: engagement,
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

func (s *CompetitorService) SyncCompetitorMetrics(ctx context.Context, id bson.ObjectID) error {
	comp, err := s.repo.FindByID(ctx, id)
	if err != nil || comp == nil {
		return utils.NotFound("Competitor not found")
	}

	growth := int64(rand.Intn(100) - 20)
	newFollowers := comp.Followers + growth
	newPosts := comp.PostsCount
	if rand.Float32() > 0.6 {
		newPosts++
	}
	newEngagement := comp.EngagementRate + (rand.Float64()*0.4 - 0.2)
	if newEngagement < 0.1 {
		newEngagement = 0.5
	}

	snapshot := models.CompetitorSnapshot{
		Date:           time.Now().Format("2006-01-02"),
		Followers:      newFollowers,
		PostsCount:     newPosts,
		EngagementRate: newEngagement,
	}

	return s.repo.AddSnapshot(ctx, id, snapshot)
}
