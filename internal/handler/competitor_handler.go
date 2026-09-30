package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type CompetitorHandler struct {
	service *service.CompetitorService
}

func NewCompetitorHandler(service *service.CompetitorService) *CompetitorHandler {
	return &CompetitorHandler{service: service}
}

func (h *CompetitorHandler) Add(c *gin.Context) {
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

	var req models.AddCompetitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	comp, err := h.service.AddCompetitor(c.Request.Context(), brandID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, utils.APIResponse{
		Success: true,
		Message: "Competitor added for benchmarking",
		Data:    comp,
	})
}

func (h *CompetitorHandler) List(c *gin.Context) {
	brandIDStr := c.Query("brandId")
	if brandIDStr == "" {
		if bid, exists := c.Get("brandId"); exists {
			brandIDStr, _ = bid.(string)
		}
	}
	brandID, err := bson.ObjectIDFromHex(brandIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Valid brandId query param is required"})
		return
	}

	comps, err := h.service.ListCompetitors(c.Request.Context(), brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    comps,
	})
}

func (h *CompetitorHandler) Delete(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid competitor ID"})
		return
	}

	brandIDStr := c.Query("brandId")
	if brandIDStr == "" {
		if bid, exists := c.Get("brandId"); exists {
			brandIDStr, _ = bid.(string)
		}
	}
	brandID, err := bson.ObjectIDFromHex(brandIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Valid brandId query param is required"})
		return
	}

	if err := h.service.DeleteCompetitor(c.Request.Context(), id, brandID); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Competitor removed successfully",
	})
}

func (h *CompetitorHandler) Sync(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid competitor ID"})
		return
	}

	if err := h.service.SyncCompetitorMetrics(c.Request.Context(), id); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Competitor metrics synced successfully",
	})
}
