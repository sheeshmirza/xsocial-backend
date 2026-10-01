package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
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
	postRepo      *repository.PostRepository
}

func NewReportService(
	repo *repository.ReportRepository,
	brandRepo *repository.BrandRepository,
	analyticsRepo *repository.AnalyticsRepository,
	postRepo *repository.PostRepository,
) *ReportService {
	return &ReportService{
		repo:          repo,
		brandRepo:     brandRepo,
		analyticsRepo: analyticsRepo,
		postRepo:      postRepo,
	}
}

func (s *ReportService) GenerateReport(ctx context.Context, userID bson.ObjectID, req models.GenerateReportRequest) (*models.Report, error) {
	brandID, err := bson.ObjectIDFromHex(req.BrandID)
	if err != nil {
		return nil, utils.BadRequest("Invalid brand ID")
	}

	// Verify membership
	member, err := s.brandRepo.FindMember(ctx, brandID, userID)
	if err != nil || member == nil {
		return nil, utils.Forbidden("Access to brand denied")
	}

	pStart, err := time.Parse(time.RFC3339, req.PeriodStart)
	if err != nil {
		pStart = time.Now().AddDate(0, -1, 0)
	}
	pEnd, err := time.Parse(time.RFC3339, req.PeriodEnd)
	if err != nil {
		pEnd = time.Now()
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = "pdf"
	}

	logo := req.WhiteLabelLogo
	if logo == "" {
		if brand, err := s.brandRepo.FindBrandByID(ctx, brandID); err == nil && brand != nil {
			logo = brand.LogoURL
		}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = fmt.Sprintf("Performance Report (%s to %s)", pStart.Format("2006-01-02"), pEnd.Format("2006-01-02"))
	}

	report := &models.Report{
		BrandID:        brandID,
		Title:          title,
		PeriodStart:    pStart,
		PeriodEnd:      pEnd,
		Format:         format,
		WhiteLabelLogo: logo,
	}

	if err := s.repo.Create(ctx, report); err != nil {
		return nil, utils.InternalServerError("Failed to create report job")
	}

	// Process report with real metrics
	s.processReport(context.Background(), report.ID, brandID, pStart, pEnd, format)

	return s.repo.FindByID(ctx, report.ID)
}

func (s *ReportService) processReport(ctx context.Context, reportID, brandID bson.ObjectID, pStart, pEnd time.Time, format string) {
	metrics := make(map[string]any)

	// 1. Gather web metrics
	if webMetrics, err := s.analyticsRepo.GetWebMetrics(ctx, brandID, pStart, pEnd); err == nil {
		metrics["web"] = webMetrics
	}

	// 2. Gather social rollups
	fromDate := pStart.Format("2006-01-02")
	toDate := pEnd.Format("2006-01-02")
	if rollups, err := s.analyticsRepo.GetSocialMetrics(ctx, brandID, "", fromDate, toDate); err == nil {
		var totalImpr, totalReach, totalEng, totalClicks int64
		for _, r := range rollups {
			totalImpr += r.Impressions
			totalReach += r.Reach
			totalEng += r.Engagement
			totalClicks += r.Clicks
		}
		metrics["social"] = map[string]any{
			"totalImpressions": totalImpr,
			"totalReach":       totalReach,
			"totalEngagement":  totalEng,
			"totalClicks":      totalClicks,
			"rollupsCount":     len(rollups),
		}
	}

	// 3. Gather post counts
	postFilter := bson.M{
		"brandId": brandID,
		"createdAt": bson.M{
			"$gte": pStart,
			"$lte": pEnd,
		},
	}
	totalPosts, _ := s.postRepo.CountByFilter(ctx, postFilter)

	postFilter["status"] = models.PostStatusPublished
	publishedPosts, _ := s.postRepo.CountByFilter(ctx, postFilter)

	postFilter["status"] = models.PostStatusScheduled
	scheduledPosts, _ := s.postRepo.CountByFilter(ctx, postFilter)

	metrics["posts"] = map[string]any{
		"total":     totalPosts,
		"published": publishedPosts,
		"scheduled": scheduledPosts,
	}

	fileURL := fmt.Sprintf("/v1/reports/%s/download", reportID.Hex())
	_ = s.repo.UpdateWithMetrics(ctx, reportID, models.ReportStatusCompleted, fileURL, metrics)
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

func (s *ReportService) BuildReportExportData(ctx context.Context, id bson.ObjectID) (title, filename, contentType string, data []byte, err error) {
	rep, err := s.repo.FindByID(ctx, id)
	if err != nil || rep == nil {
		return "", "", "", nil, utils.NotFound("Report not found")
	}

	brandName := "Brand"
	if brand, bErr := s.brandRepo.FindBrandByID(ctx, rep.BrandID); bErr == nil && brand != nil {
		brandName = brand.Name
	}

	filenameBase := fmt.Sprintf("report-%s-%s", utils.Slugify(brandName), rep.ID.Hex()[:8])

	switch strings.ToLower(rep.Format) {
	case "csv":
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"Category", "Metric", "Value", "PeriodStart", "PeriodEnd"})
		_ = w.Write([]string{"General", "Brand Name", brandName, rep.PeriodStart.Format("2006-01-02"), rep.PeriodEnd.Format("2006-01-02")})
		_ = w.Write([]string{"General", "Report Title", rep.Title, rep.PeriodStart.Format("2006-01-02"), rep.PeriodEnd.Format("2006-01-02")})

		if m, ok := rep.Metrics["web"].(map[string]any); ok {
			for k, v := range m {
				_ = w.Write([]string{"Web Analytics", k, fmt.Sprintf("%v", v), rep.PeriodStart.Format("2006-01-02"), rep.PeriodEnd.Format("2006-01-02")})
			}
		}
		if m, ok := rep.Metrics["social"].(map[string]any); ok {
			for k, v := range m {
				_ = w.Write([]string{"Social Metrics", k, fmt.Sprintf("%v", v), rep.PeriodStart.Format("2006-01-02"), rep.PeriodEnd.Format("2006-01-02")})
			}
		}
		if m, ok := rep.Metrics["posts"].(map[string]any); ok {
			for k, v := range m {
				_ = w.Write([]string{"Posts Activity", k, fmt.Sprintf("%v", v), rep.PeriodStart.Format("2006-01-02"), rep.PeriodEnd.Format("2006-01-02")})
			}
		}
		w.Flush()
		return rep.Title, filenameBase + ".csv", "text/csv", buf.Bytes(), nil

	case "json":
		exportDoc := map[string]any{
			"reportId":    rep.ID.Hex(),
			"title":       rep.Title,
			"brandId":     rep.BrandID.Hex(),
			"brandName":   brandName,
			"periodStart": rep.PeriodStart.Format(time.RFC3339),
			"periodEnd":   rep.PeriodEnd.Format(time.RFC3339),
			"metrics":     rep.Metrics,
			"generatedAt": time.Now().Format(time.RFC3339),
		}
		b, _ := json.MarshalIndent(exportDoc, "", "  ")
		return rep.Title, filenameBase + ".json", "application/json", b, nil

	default: // pdf or text formatted summary
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("====================================================\n"))
		buf.WriteString(fmt.Sprintf("             %s\n", strings.ToUpper(rep.Title)))
		buf.WriteString(fmt.Sprintf("====================================================\n\n"))
		buf.WriteString(fmt.Sprintf("Brand:        %s\n", brandName))
		buf.WriteString(fmt.Sprintf("Date Range:   %s to %s\n", rep.PeriodStart.Format("2006-01-02"), rep.PeriodEnd.Format("2006-01-02")))
		buf.WriteString(fmt.Sprintf("Generated At: %s\n\n", time.Now().Format(time.RFC850)))

		buf.WriteString("[WEB ANALYTICS]\n")
		if m, ok := rep.Metrics["web"].(map[string]any); ok {
			for k, v := range m {
				buf.WriteString(fmt.Sprintf(" - %s: %v\n", k, v))
			}
		} else {
			buf.WriteString(" - No web traffic recorded in period.\n")
		}
		buf.WriteString("\n")

		buf.WriteString("[SOCIAL METRICS]\n")
		if m, ok := rep.Metrics["social"].(map[string]any); ok {
			for k, v := range m {
				buf.WriteString(fmt.Sprintf(" - %s: %v\n", k, v))
			}
		} else {
			buf.WriteString(" - No social metric rollups recorded in period.\n")
		}
		buf.WriteString("\n")

		buf.WriteString("[POSTING ACTIVITY]\n")
		if m, ok := rep.Metrics["posts"].(map[string]any); ok {
			for k, v := range m {
				buf.WriteString(fmt.Sprintf(" - %s: %v\n", k, v))
			}
		} else {
			buf.WriteString(" - No posts created during this period.\n")
		}
		buf.WriteString("\n====================================================\n")

		return rep.Title, filenameBase + ".txt", "text/plain; charset=utf-8", buf.Bytes(), nil
	}
}
