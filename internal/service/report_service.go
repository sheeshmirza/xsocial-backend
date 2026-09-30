package service

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type ReportService struct {
	repo          *repository.ReportRepository
	brandRepo     *repository.BrandRepository
	analyticsRepo *repository.AnalyticsRepository
}

func NewReportService(repo *repository.ReportRepository, brandRepo *repository.BrandRepository, analyticsRepo *repository.AnalyticsRepository) *ReportService {
	return &ReportService{
		repo:          repo,
		brandRepo:     brandRepo,
		analyticsRepo: analyticsRepo,
	}
}

func (s *ReportService) GenerateReport(ctx context.Context, userID bson.ObjectID, req models.GenerateReportRequest) (*models.Report, error) {
	brandID, err := bson.ObjectIDFromHex(req.BrandID)
	if err != nil {
		return nil, utils.BadRequest("Invalid brand ID")
	}

	pStart, err := time.Parse(time.RFC3339, req.PeriodStart)
	if err != nil {
		pStart = time.Now().AddDate(0, -1, 0)
	}
	pEnd, err := time.Parse(time.RFC3339, req.PeriodEnd)
	if err != nil {
		pEnd = time.Now()
	}

	format := req.Format
	if format == "" {
		format = "pdf"
	}

	logo := req.WhiteLabelLogo
	if logo == "" {
		if brand, err := s.brandRepo.FindBrandByID(ctx, brandID); err == nil && brand != nil {
			logo = brand.LogoURL
		}
	}

	report := &models.Report{
		BrandID:        brandID,
		Title:          req.Title,
		PeriodStart:    pStart,
		PeriodEnd:      pEnd,
		Format:         format,
		WhiteLabelLogo: logo,
	}

	if err := s.repo.Create(ctx, report); err != nil {
		return nil, utils.InternalServerError("Failed to create report job")
	}

	// Process report asynchronously
	go s.processReport(report.ID, brandID, format)

	return report, nil
}

func (s *ReportService) processReport(reportID, brandID bson.ObjectID, format string) {
	// In production, render PDF/CSV via headless Chrome / PDF generator
	// Generate mock report file URL
	fileURL := fmt.Sprintf("/api/v1/reports/%s/download.%s", reportID.Hex(), format)
	_ = s.repo.UpdateStatus(context.Background(), reportID, models.ReportStatusCompleted, fileURL)
}

func (s *ReportService) ListReports(ctx context.Context, brandID bson.ObjectID) ([]models.Report, error) {
	return s.repo.FindByBrand(ctx, brandID)
}

func (s *ReportService) GetReport(ctx context.Context, id bson.ObjectID) (*models.Report, error) {
	rep, err := s.repo.FindByID(ctx, id)
	if err != nil || rep == nil {
		return nil, utils.NotFound("Report not found")
	}
	return rep, nil
}
