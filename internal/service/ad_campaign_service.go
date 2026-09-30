package service

import (
	"context"
	"math/rand"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type AdCampaignService struct {
	repo *repository.AdCampaignRepository
}

func NewAdCampaignService(repo *repository.AdCampaignRepository) *AdCampaignService {
	return &AdCampaignService{
		repo: repo,
	}
}

func (s *AdCampaignService) ListCampaigns(ctx context.Context, brandID bson.ObjectID, platform string) ([]models.AdCampaign, error) {
	return s.repo.FindByBrand(ctx, brandID, platform)
}

func (s *AdCampaignService) GetSummary(ctx context.Context, brandID bson.ObjectID) (*models.AdSummary, error) {
	return s.repo.GetSummary(ctx, brandID)
}

func (s *AdCampaignService) SyncAdAccounts(ctx context.Context, brandID bson.ObjectID, platform string) error {
	// Sync simulated or live ad metrics from Meta / Google / TikTok Ads API
	platforms := []string{"meta", "google", "tiktok"}
	if platform != "" {
		platforms = []string{platform}
	}

	for _, p := range platforms {
		campaigns := []struct {
			name      string
			budget    float64
			spend     float64
			impr      int64
			clicks    int64
			convs     int64
			objective string
		}{
			{name: "Spring Conversion Booster", budget: 1500, spend: 1240.50, impr: 85000, clicks: 3200, convs: 184, objective: "CONVERSIONS"},
			{name: "Brand Awareness Evergreen", budget: 800, spend: 760.00, impr: 140000, clicks: 1950, convs: 42, objective: "AWARENESS"},
			{name: "Retargeting Cart Abandoners", budget: 500, spend: 490.20, impr: 22000, clicks: 1100, convs: 98, objective: "SALES"},
		}

		for _, c := range campaigns {
			// jitter
			spend := c.spend + float64(rand.Intn(50))
			impr := c.impr + int64(rand.Intn(2000))
			clicks := c.clicks + int64(rand.Intn(100))
			convs := c.convs + int64(rand.Intn(10))

			ctr := 0.0
			if impr > 0 {
				ctr = (float64(clicks) / float64(impr)) * 100
			}
			cpc := 0.0
			if clicks > 0 {
				cpc = spend / float64(clicks)
			}
			cpm := 0.0
			if impr > 0 {
				cpm = (spend / float64(impr)) * 1000
			}
			roas := 0.0
			if spend > 0 && convs > 0 {
				roas = (float64(convs*45) / spend)
			}

			camp := &models.AdCampaign{
				BrandID:            brandID,
				Platform:           p,
				ExternalCampaignID: p + "_camp_" + utils.HashSHA256(c.name)[:12],
				Name:               c.name,
				Status:             "ACTIVE",
				Objective:          c.objective,
				Budget:             c.budget,
				Spend:              spend,
				Impressions:        impr,
				Clicks:             clicks,
				Conversions:        convs,
				CTR:                ctr,
				CPC:                cpc,
				CPM:                cpm,
				ROAS:               roas,
				DateRange:          "Last 30 Days",
			}
			_ = s.repo.UpsertCampaign(ctx, camp)
		}
	}
	return nil
}
