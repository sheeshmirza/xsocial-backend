package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type WebhookHandler struct {
	service *service.WebhookService
}

func NewWebhookHandler(service *service.WebhookService) *WebhookHandler {
	return &WebhookHandler{service: service}
}

func (h *WebhookHandler) Create(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, ok := userVal.(*models.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Success: false, Message: "Unauthorized"})
		return
	}

	var req models.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid request payload", Errors: err.Error()})
		return
	}

	wh, err := h.service.CreateWebhook(c.Request.Context(), user.ID, req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, utils.APIResponse{
		Success: true,
		Message: "Webhook endpoint registered successfully",
		Data:    wh,
	})
}

func (h *WebhookHandler) List(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, ok := userVal.(*models.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Success: false, Message: "Unauthorized"})
		return
	}

	var brandOID *bson.ObjectID
	if bid := c.Query("brandId"); bid != "" {
		if id, err := bson.ObjectIDFromHex(bid); err == nil {
			brandOID = &id
		}
	}

	list, err := h.service.ListWebhooks(c.Request.Context(), user.ID, brandOID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    list,
	})
}

func (h *WebhookHandler) Delete(c *gin.Context) {
	userVal, _ := c.Get("user")
	user, ok := userVal.(*models.User)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Success: false, Message: "Unauthorized"})
		return
	}

	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid webhook ID"})
		return
	}

	if err := h.service.DeleteWebhook(c.Request.Context(), id, user.ID); err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Message: "Webhook deleted successfully",
	})
}
