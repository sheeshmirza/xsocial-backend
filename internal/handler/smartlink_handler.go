package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type SmartLinkHandler struct {
	service *service.SmartLinkService
}

func NewSmartLinkHandler(service *service.SmartLinkService) *SmartLinkHandler {
	return &SmartLinkHandler{service: service}
}

func (h *SmartLinkHandler) Create(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, ok := userVal.(*models.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Success: false, Message: "Unauthorized"})
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
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Valid brandId is required"})
		return
	}

	var req models.CreateSmartLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	page, err := h.service.CreatePage(c.Request.Context(), brandID, user.ID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, utils.APIResponse{
		Success: true,
		Message: "SmartLink page created successfully",
		Data:    page,
	})
}

func (h *SmartLinkHandler) List(c *gin.Context) {
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

	pages, err := h.service.ListPagesByBrand(c.Request.Context(), brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    pages,
	})
}

func (h *SmartLinkHandler) Update(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid page ID"})
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

	var req models.UpdateSmartLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	updated, err := h.service.UpdatePage(c.Request.Context(), id, brandID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "SmartLink page updated successfully",
		Data:    updated,
	})
}

func (h *SmartLinkHandler) Delete(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid page ID"})
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

	if err := h.service.DeletePage(c.Request.Context(), id, brandID); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "SmartLink page deleted successfully",
	})
}

func (h *SmartLinkHandler) Analytics(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid page ID"})
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

	stats, err := h.service.GetAnalytics(c.Request.Context(), id, brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    stats,
	})
}

// PublicBioPage serves public link-in-bio view & registers view analytics
func (h *SmartLinkHandler) PublicBioPage(c *gin.Context) {
	slug := c.Param("slug")
	page, err := h.service.GetPageBySlug(c.Request.Context(), slug, true)
	if err != nil {
		c.JSON(http.StatusNotFound, utils.APIResponse{Success: false, Message: "Bio page not found"})
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    page,
	})
}

// PublicTrackClick registers a button click on a public bio page
func (h *SmartLinkHandler) PublicTrackClick(c *gin.Context) {
	slug := c.Param("slug")
	buttonID := c.Param("buttonId")

	_ = h.service.TrackButtonClick(c.Request.Context(), slug, buttonID)
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
