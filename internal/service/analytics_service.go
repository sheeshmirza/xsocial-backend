package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type AnalyticsService struct {
	repo      *repository.AnalyticsRepository
	brandRepo *repository.BrandRepository
}

func NewAnalyticsService(repo *repository.AnalyticsRepository, brandRepo *repository.BrandRepository) *AnalyticsService {
	return &AnalyticsService{
		repo:      repo,
		brandRepo: brandRepo,
	}
}

func (s *AnalyticsService) RecordWebHit(ctx context.Context, brandID bson.ObjectID, sessionID, url, path, ref, country, device, browser string) error {
	event := &models.WebEvent{
		BrandID:   brandID,
		SessionID: sessionID,
		URL:       url,
		Path:      path,
		Referrer:  ref,
		Country:   country,
		Device:    device,
		Browser:   browser,
		Timestamp: time.Now(),
	}
	return s.repo.RecordWebEvent(ctx, event)
}

func (s *AnalyticsService) GetWebOverview(ctx context.Context, brandID bson.ObjectID, days int) (map[string]any, error) {
	if days <= 0 {
		days = 30
	}
	to := time.Now()
	from := to.AddDate(0, 0, -days)

	return s.repo.GetWebMetrics(ctx, brandID, from, to)
}

func (s *AnalyticsService) GetSocialOverview(ctx context.Context, brandID bson.ObjectID, platform string, days int) ([]models.SocialMetricRollup, error) {
	if days <= 0 {
		days = 30
	}
	to := time.Now()
	from := to.AddDate(0, 0, -days)

	fromDate := from.Format("2006-01-02")
	toDate := to.Format("2006-01-02")

	rollups, err := s.repo.GetSocialMetrics(ctx, brandID, platform, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	if rollups == nil {
		rollups = []models.SocialMetricRollup{}
	}
	return rollups, nil
}

func (s *AnalyticsService) RecordSocialMetricRollup(ctx context.Context, rollup *models.SocialMetricRollup) error {
	if rollup == nil || rollup.BrandID.IsZero() {
		return utils.BadRequest("invalid rollup payload or missing brandId")
	}
	if rollup.Date == "" {
		rollup.Date = time.Now().Format("2006-01-02")
	}
	return s.repo.UpsertSocialRollup(ctx, rollup)
}
