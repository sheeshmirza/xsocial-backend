package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type AdCampaignHandler struct {
	service *service.AdCampaignService
}

func NewAdCampaignHandler(service *service.AdCampaignService) *AdCampaignHandler {
	return &AdCampaignHandler{service: service}
}

func (h *AdCampaignHandler) List(c *gin.Context) {
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

	platform := c.Query("platform")
	camps, err := h.service.ListCampaigns(c.Request.Context(), brandID, platform)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    camps,
	})
}

func (h *AdCampaignHandler) Summary(c *gin.Context) {
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

	summary, err := h.service.GetSummary(c.Request.Context(), brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    summary,
	})
}

func (h *AdCampaignHandler) Sync(c *gin.Context) {
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

	platform := c.Query("platform")
	if err := h.service.SyncAdAccounts(c.Request.Context(), brandID, platform); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Ad campaigns synchronized successfully",
	})
}
