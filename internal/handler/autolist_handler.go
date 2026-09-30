package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type AutolistHandler struct {
	service *service.AutolistService
}

func NewAutolistHandler(service *service.AutolistService) *AutolistHandler {
	return &AutolistHandler{service: service}
}

func (h *AutolistHandler) Create(c *gin.Context) {
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

	var req models.CreateAutolistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	list, err := h.service.CreateAutolist(c.Request.Context(), brandID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, utils.APIResponse{
		Success: true,
		Message: "Autolist created successfully",
		Data:    list,
	})
}

func (h *AutolistHandler) List(c *gin.Context) {
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

	lists, err := h.service.ListAutolists(c.Request.Context(), brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    lists,
	})
}

func (h *AutolistHandler) AddPost(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid autolist ID"})
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

	var req models.AddAutolistPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	updated, err := h.service.AddPostToAutolist(c.Request.Context(), id, brandID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Post added to evergreen queue",
		Data:    updated,
	})
}

func (h *AutolistHandler) Delete(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid autolist ID"})
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

	if err := h.service.DeleteAutolist(c.Request.Context(), id, brandID); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Autolist deleted successfully",
	})
}

// RSS Feeds
func (h *AutolistHandler) CreateRSSFeed(c *gin.Context) {
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

	var req models.CreateRSSFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	feed, err := h.service.CreateRSSFeed(c.Request.Context(), brandID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, utils.APIResponse{
		Success: true,
		Message: "RSS feed registered successfully",
		Data:    feed,
	})
}

func (h *AutolistHandler) ListRSSFeeds(c *gin.Context) {
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

	feeds, err := h.service.ListRSSFeeds(c.Request.Context(), brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    feeds,
	})
}

func (h *AutolistHandler) DeleteRSSFeed(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid feed ID"})
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

	if err := h.service.DeleteRSSFeed(c.Request.Context(), id, brandID); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "RSS feed deleted successfully",
	})
}
