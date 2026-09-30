package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type InboxHandler struct {
	service *service.InboxService
}

func NewInboxHandler(service *service.InboxService) *InboxHandler {
	return &InboxHandler{service: service}
}

func (h *InboxHandler) List(c *gin.Context) {
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

	status := c.Query("status")
	platform := c.Query("platform")
	page, _ := strconv.ParseInt(c.DefaultQuery("page", "1"), 10, 64)
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "20"), 10, 64)

	convs, total, err := h.service.ListConversations(c.Request.Context(), brandID, user.ID, status, platform, page, limit)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data: gin.H{
			"conversations": convs,
			"total":         total,
			"page":          page,
			"limit":         limit,
		},
	})
}

func (h *InboxHandler) Get(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid conversation ID"})
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

	conv, msgs, err := h.service.GetConversation(c.Request.Context(), id, brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data: gin.H{
			"conversation": conv,
			"messages":     msgs,
		},
	})
}

func (h *InboxHandler) Reply(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, ok := userVal.(*models.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Success: false, Message: "Unauthorized"})
		return
	}

	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid conversation ID"})
		return
	}

	var req models.ReplyInboxRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
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

	msg, err := h.service.ReplyToConversation(c.Request.Context(), id, brandID, user.ID, user.FullName, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Reply sent successfully",
		Data:    msg,
	})
}

func (h *InboxHandler) UpdateStatus(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid conversation ID"})
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

	var req models.UpdateInboxStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	updated, err := h.service.UpdateConversationStatus(c.Request.Context(), id, brandID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Conversation updated successfully",
		Data:    updated,
	})
}

func (h *InboxHandler) ListCannedReplies(c *gin.Context) {
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

	replies, err := h.service.ListCannedReplies(c.Request.Context(), brandID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    replies,
	})
}

func (h *InboxHandler) CreateCannedReply(c *gin.Context) {
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

	var req struct {
		Shortcut string `json:"shortcut" binding:"required"`
		Title    string `json:"title" binding:"required"`
		Content  string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	reply, err := h.service.CreateCannedReply(c.Request.Context(), brandID, req.Shortcut, req.Title, req.Content)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, utils.APIResponse{
		Success: true,
		Message: "Canned reply created successfully",
		Data:    reply,
	})
}

func (h *InboxHandler) DeleteCannedReply(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid canned reply ID"})
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

	if err := h.service.DeleteCannedReply(c.Request.Context(), id, brandID); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Canned reply deleted successfully",
	})
}
