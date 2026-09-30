package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type ReportHandler struct {
	service *service.ReportService
}

func NewReportHandler(service *service.ReportService) *ReportHandler {
	return &ReportHandler{service: service}
}

func (h *ReportHandler) Generate(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, ok := userVal.(*models.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Success: false, Message: "Unauthorized"})
		return
	}

	var req models.GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	report, err := h.service.GenerateReport(c.Request.Context(), user.ID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusAccepted, utils.APIResponse{
		Success: true,
		Message: "Report generation started",
		Data:    report,
	})
}

func (h *ReportHandler) List(c *gin.Context) {
	brandIDStr := c.Query("brandId")
	if brandIDStr == "" {
		if bid, exists := c.Get("brandId"); exists {
			brandIDStr, _ = bid.(string)
		}
	}
	brandID, err := bson.ObjectIDFromHex(brandIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Valid brandId is required"})
		return
	}

	reports, err := h.service.ListReports(c.Request.Context(), brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    reports,
	})
}

func (h *ReportHandler) Download(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid report ID"})
		return
	}

	report, err := h.service.GetReport(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	// Serve generated CSV / PDF summary content
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"report-%s.%s\"", report.ID.Hex(), report.Format))
	c.Header("Content-Type", "text/plain")
	c.String(http.StatusOK, fmt.Sprintf("XSocial Report: %s\nPeriod: %s - %s\nStatus: %s\nBrand: %s\n",
		report.Title, report.PeriodStart.Format("2006-01-02"), report.PeriodEnd.Format("2006-01-02"), report.Status, report.BrandID.Hex()))
}
