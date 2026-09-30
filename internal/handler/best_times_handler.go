package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type BestTimesHandler struct {
	service *service.BestTimesService
}

func NewBestTimesHandler(service *service.BestTimesService) *BestTimesHandler {
	return &BestTimesHandler{service: service}
}

func (h *BestTimesHandler) GetBestTimes(c *gin.Context) {
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

	resp, err := h.service.GetBestTimes(c.Request.Context(), brandID, platform)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    resp,
	})
}
