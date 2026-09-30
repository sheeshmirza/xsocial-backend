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

type AuditLogHandler struct {
	service *service.AuditLogService
}

func NewAuditLogHandler(service *service.AuditLogService) *AuditLogHandler {
	return &AuditLogHandler{service: service}
}

func (h *AuditLogHandler) List(c *gin.Context) {
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

	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "50"), 10, 64)
	logs, err := h.service.ListLogs(c.Request.Context(), &user.ID, brandOID, limit)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, utils.APIResponse{
		Success: true,
		Data:    logs,
	})
}
