package service

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type AdCampaignService struct {
	repo       *repository.AdCampaignRepository
	handleRepo *repository.SocialHandleRepository
	brandRepo  *repository.BrandRepository
}

func NewAdCampaignService(
	repo *repository.AdCampaignRepository,
	handleRepo *repository.SocialHandleRepository,
	brandRepo *repository.BrandRepository,
) *AdCampaignService {
	return &AdCampaignService{
		repo:       repo,
		handleRepo: handleRepo,
		brandRepo:  brandRepo,
	}
}

func (s *AdCampaignService) ListCampaigns(ctx context.Context, brandID bson.ObjectID, platform string) ([]models.AdCampaign, error) {
	return s.repo.FindByBrand(ctx, brandID, platform)
}

func (s *AdCampaignService) GetSummary(ctx context.Context, brandID bson.ObjectID) (*models.AdSummary, error) {
	return s.repo.GetSummary(ctx, brandID)
}

func (s *AdCampaignService) CreateCampaign(ctx context.Context, brandID bson.ObjectID, req models.CreateAdCampaignRequest) (*models.AdCampaign, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, utils.BadRequest("campaign name is required")
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform == "" {
		return nil, utils.BadRequest("platform is required (meta, google, tiktok)")
	}

	extID := strings.TrimSpace(req.ExternalCampaignID)
	if extID == "" {
		extID = platform + "_camp_" + utils.HashSHA256(name + brandID.Hex())[:12]
	}

	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status == "" {
		status = "ACTIVE"
	}

	ctr := 0.0
	if req.Impressions > 0 {
		ctr = (float64(req.Clicks) / float64(req.Impressions)) * 100
	}
	cpc := 0.0
	if req.Clicks > 0 {
		cpc = req.Spend / float64(req.Clicks)
	}
	cpm := 0.0
	if req.Impressions > 0 {
		cpm = (req.Spend / float64(req.Impressions)) * 1000
	}
	roas := 0.0
	if req.Spend > 0 && req.Conversions > 0 {
		roas = float64(req.Conversions*50) / req.Spend
	}

	dateRange := strings.TrimSpace(req.DateRange)
	if dateRange == "" {
		dateRange = "Last 30 Days"
	}

	camp := &models.AdCampaign{
		BrandID:            brandID,
		Platform:           platform,
		ExternalCampaignID: extID,
		Name:               name,
		Status:             status,
		Objective:          req.Objective,
		Budget:             req.Budget,
		Spend:              req.Spend,
		Impressions:        req.Impressions,
		Clicks:             req.Clicks,
		Conversions:        req.Conversions,
		CTR:                ctr,
		CPC:                cpc,
		CPM:                cpm,
		ROAS:               roas,
		DateRange:          dateRange,
	}

	if err := s.repo.UpsertCampaign(ctx, camp); err != nil {
		return nil, utils.InternalServerError("Failed to save ad campaign")
	}

	return camp, nil
}

func (s *AdCampaignService) UpdateCampaign(ctx context.Context, id, brandID bson.ObjectID, req models.UpdateAdCampaignRequest) (*models.AdCampaign, error) {
	update := bson.M{}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" {
			return nil, utils.BadRequest("campaign name cannot be empty")
		}
		update["name"] = n
	}
	if req.Status != nil {
		update["status"] = strings.ToUpper(strings.TrimSpace(*req.Status))
	}
	if req.Objective != nil {
		update["objective"] = *req.Objective
	}
	if req.Budget != nil {
		update["budget"] = *req.Budget
	}
	if req.Spend != nil {
		update["spend"] = *req.Spend
	}
	if req.Impressions != nil {
		update["impressions"] = *req.Impressions
	}
	if req.Clicks != nil {
		update["clicks"] = *req.Clicks
	}
	if req.Conversions != nil {
		update["conversions"] = *req.Conversions
	}
	if req.DateRange != nil {
		update["dateRange"] = *req.DateRange
	}

	// Calculate derived metrics if spend/impressions/clicks changed
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil || existing == nil {
		return nil, utils.NotFound("Ad campaign not found")
	}

	impr := existing.Impressions
	if req.Impressions != nil {
		impr = *req.Impressions
	}
	clicks := existing.Clicks
	if req.Clicks != nil {
		clicks = *req.Clicks
	}
	spend := existing.Spend
	if req.Spend != nil {
		spend = *req.Spend
	}
	convs := existing.Conversions
	if req.Conversions != nil {
		convs = *req.Conversions
	}

	if impr > 0 {
		update["ctr"] = (float64(clicks) / float64(impr)) * 100
		update["cpm"] = (spend / float64(impr)) * 1000
	}
	if clicks > 0 {
		update["cpc"] = spend / float64(clicks)
	}
	if spend > 0 && convs > 0 {
		update["roas"] = float64(convs*50) / spend
	}

	updated, err := s.repo.Update(ctx, id, brandID, update)
	if err != nil {
		return nil, utils.InternalServerError("Failed to update ad campaign")
	}
	if updated == nil {
		return nil, utils.NotFound("Ad campaign not found")
	}
	return updated, nil
}

func (s *AdCampaignService) DeleteCampaign(ctx context.Context, id, brandID bson.ObjectID) error {
	deleted, err := s.repo.Delete(ctx, id, brandID)
	if err != nil {
		return utils.InternalServerError("Failed to delete ad campaign")
	}
	if !deleted {
		return utils.NotFound("Ad campaign not found")
	}
	return nil
}

func (s *AdCampaignService) SyncAdAccounts(ctx context.Context, brandID bson.ObjectID, platform string) error {
	// Query connected handles for the brand to sync real ad metrics
	handles, err := s.handleRepo.FindByBrand(ctx, brandID)
	if err != nil {
		return utils.InternalServerError("Failed to query connected handles")
	}

	// Only sync platforms where valid credentials exist
	platformMap := make(map[string]*models.SocialHandle)
	for i := range handles {
		h := &handles[i]
		if h.IsConnected && !h.IsExpired() {
			platformMap[strings.ToLower(h.Platform)] = h
		}
	}

	requestedPlatforms := []string{"meta", "google", "tiktok"}
	if platform != "" {
		requestedPlatforms = []string{strings.ToLower(platform)}
	}

	for _, p := range requestedPlatforms {
		handle, ok := platformMap[p]
		if !ok || handle.AccessToken == "" {
			// No connected ad account for this platform; skip without fabricating fake data
			continue
		}
		// Future: Call platform's real Ad API (Meta Marketing API / Google Ads API) using handle.AccessToken
	}

	return nil
}
