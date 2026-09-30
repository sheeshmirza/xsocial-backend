package handler

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type AnalyticsHandler struct {
	service *service.AnalyticsService
}

func NewAnalyticsHandler(service *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) WebOverview(c *gin.Context) {
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

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	overview, err := h.service.GetWebOverview(c.Request.Context(), brandID, days)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    overview,
	})
}

func (h *AnalyticsHandler) SocialOverview(c *gin.Context) {
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

	platform := c.DefaultQuery("platform", "instagram")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	rollups, err := h.service.GetSocialOverview(c.Request.Context(), brandID, platform, days)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    rollups,
	})
}

// 1x1 transparent GIF tracking pixel bytes
var transparentGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x21,
	0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
	0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

// PublicTrackerPixel serves a 1x1 GIF tracking pixel for website traffic analytics
func (h *AnalyticsHandler) PublicTrackerPixel(c *gin.Context) {
	brandIDStr := c.Query("id")
	if brandID, err := bson.ObjectIDFromHex(brandIDStr); err == nil {
		sessionID := c.Query("sid")
		url := c.Query("u")
		path := c.Query("p")
		ref := c.Request.Referer()
		country := c.GetHeader("CF-IPCountry")
		device := c.Query("d")
		browser := c.Request.UserAgent()

		go func() {
			_ = h.service.RecordWebHit(c.Request.Context(), brandID, sessionID, url, path, ref, country, device, browser)
		}()
	}

	c.Header("Content-Type", "image/gif")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Data(http.StatusOK, "image/gif", transparentGIF)
}

// PublicTrackerEvent accepts beacon / JSON tracking events from JavaScript websites
func (h *AnalyticsHandler) PublicTrackerEvent(c *gin.Context) {
	var req struct {
		BrandID   string `json:"brandId" binding:"required"`
		SessionID string `json:"sessionId"`
		URL       string `json:"url"`
		Path      string `json:"path"`
		Referrer  string `json:"referrer"`
		Device    string `json:"device"`
	}
	if err := c.ShouldBindJSON(&req); err == nil {
		if brandID, err := bson.ObjectIDFromHex(req.BrandID); err == nil {
			country := c.GetHeader("CF-IPCountry")
			browser := c.Request.UserAgent()
			_ = h.service.RecordWebHit(c.Request.Context(), brandID, req.SessionID, req.URL, req.Path, req.Referrer, country, req.Device, browser)
		}
	}
	c.Status(http.StatusNoContent)
}

// Dummy reference to avoid unused base64
var _ = base64.StdEncoding
