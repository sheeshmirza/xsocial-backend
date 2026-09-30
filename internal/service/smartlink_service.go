package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type SmartLinkService struct {
	repo      *repository.SmartLinkRepository
	brandRepo *repository.BrandRepository
}

func NewSmartLinkService(repo *repository.SmartLinkRepository, brandRepo *repository.BrandRepository) *SmartLinkService {
	return &SmartLinkService{
		repo:      repo,
		brandRepo: brandRepo,
	}
}

func (s *SmartLinkService) CreatePage(ctx context.Context, brandID, userID bson.ObjectID, req models.CreateSmartLinkRequest) (*models.SmartLinkPage, error) {
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if slug == "" {
		slug = strings.ToLower(strings.ReplaceAll(req.Title, " ", "-"))
	}

	// Verify slug availability
	existing, _ := s.repo.FindPageBySlug(ctx, slug)
	if existing != nil {
		return nil, utils.Conflict("SmartLink slug is already taken")
	}

	buttons := req.Buttons
	if buttons == nil {
		buttons = []models.SmartLinkButton{}
	}
	for i := range buttons {
		if buttons[i].ID == "" {
			buttons[i].ID = uuid.NewString()
		}
		buttons[i].Order = i
		buttons[i].IsActive = true
	}

	theme := req.Theme
	if theme == "" {
		theme = "gradient"
	}

	page := &models.SmartLinkPage{
		BrandID:      brandID,
		Slug:         slug,
		Title:        req.Title,
		Bio:          req.Bio,
		AvatarURL:    req.AvatarURL,
		Theme:        theme,
		CustomColors: req.CustomColors,
		SocialLinks:  req.SocialLinks,
		Buttons:      buttons,
	}

	if err := s.repo.CreatePage(ctx, page); err != nil {
		return nil, utils.InternalServerError("Failed to create SmartLink page")
	}
	return page, nil
}

func (s *SmartLinkService) GetPageBySlug(ctx context.Context, slug string, trackView bool) (*models.SmartLinkPage, error) {
	page, err := s.repo.FindPageBySlug(ctx, slug)
	if err != nil {
		return nil, utils.InternalServerError("Failed to get SmartLink page")
	}
	if page == nil {
		return nil, utils.NotFound("SmartLink page not found")
	}

	if trackView {
		go s.repo.IncrementViews(context.Background(), slug)
	}

	return page, nil
}

func (s *SmartLinkService) TrackButtonClick(ctx context.Context, slug, buttonID string) error {
	s.repo.IncrementClicks(ctx, slug, buttonID)
	return nil
}

func (s *SmartLinkService) ListPagesByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.SmartLinkPage, error) {
	return s.repo.FindPagesByBrand(ctx, brandID)
}

func (s *SmartLinkService) UpdatePage(ctx context.Context, id, brandID bson.ObjectID, req models.UpdateSmartLinkRequest) (*models.SmartLinkPage, error) {
	page, err := s.repo.FindPageByID(ctx, id)
	if err != nil || page == nil || page.BrandID != brandID {
		return nil, utils.NotFound("SmartLink page not found")
	}

	update := bson.M{}
	if req.Title != nil {
		update["title"] = *req.Title
	}
	if req.Bio != nil {
		update["bio"] = *req.Bio
	}
	if req.AvatarURL != nil {
		update["avatarUrl"] = *req.AvatarURL
	}
	if req.Theme != nil {
		update["theme"] = *req.Theme
	}
	if req.CustomColors != nil {
		update["customColors"] = req.CustomColors
	}
	if req.SocialLinks != nil {
		update["socialLinks"] = req.SocialLinks
	}
	if req.Buttons != nil {
		btns := *req.Buttons
		for i := range btns {
			if btns[i].ID == "" {
				btns[i].ID = uuid.NewString()
			}
			btns[i].Order = i
		}
		update["buttons"] = btns
	}
	if req.IsActive != nil {
		update["isActive"] = *req.IsActive
	}

	if len(update) == 0 {
		return page, nil
	}

	return s.repo.UpdatePage(ctx, id, update)
}

func (s *SmartLinkService) DeletePage(ctx context.Context, id, brandID bson.ObjectID) error {
	deleted, err := s.repo.DeletePage(ctx, id, brandID)
	if err != nil {
		return utils.InternalServerError("Failed to delete SmartLink page")
	}
	if !deleted {
		return utils.NotFound("SmartLink page not found")
	}
	return nil
}

func (s *SmartLinkService) GetAnalytics(ctx context.Context, id, brandID bson.ObjectID) (map[string]any, error) {
	page, err := s.repo.FindPageByID(ctx, id)
	if err != nil || page == nil || page.BrandID != brandID {
		return nil, utils.NotFound("SmartLink page not found")
	}

	ctr := 0.0
	if page.TotalViews > 0 {
		ctr = (float64(page.TotalClicks) / float64(page.TotalViews)) * 100.0
	}

	buttonStats := make([]map[string]any, len(page.Buttons))
	for i, b := range page.Buttons {
		bCtr := 0.0
		if page.TotalViews > 0 {
			bCtr = (float64(b.Clicks) / float64(page.TotalViews)) * 100.0
		}
		buttonStats[i] = map[string]any{
			"id":     b.ID,
			"title":  b.Title,
			"url":    b.URL,
			"clicks": b.Clicks,
			"ctr":    bCtr,
		}
	}

	return map[string]any{
		"pageId":      page.ID.Hex(),
		"slug":        page.Slug,
		"totalViews":  page.TotalViews,
		"totalClicks": page.TotalClicks,
		"ctr":         ctr,
		"buttons":     buttonStats,
	}, nil
}
