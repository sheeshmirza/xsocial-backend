package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/platform/instagram"
	"xsocial-backend/internal/platform/linkedin"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type SocialHandler struct {
	socialService    *service.SocialService
	instagramHandler *instagram.Handler
	linkedinHandler  *linkedin.Handler
	jwtAccessSecret  string
}

func NewSocialHandler(
	socialService *service.SocialService,
	igHandler *instagram.Handler,
	liHandler *linkedin.Handler,
	jwtAccessSecret string,
) *SocialHandler {
	return &SocialHandler{
		socialService:    socialService,
		instagramHandler: igHandler,
		linkedinHandler:  liHandler,
		jwtAccessSecret:  jwtAccessSecret,
	}
}

func (h *SocialHandler) ListAccounts(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	page, limit, skip := utils.ParsePagination(c.Query("page"), c.Query("limit"))
	platformFilter := c.Query("platform")

	sortBy := c.Query("sortBy")
	if sortBy == "" {
		sortBy = "createdAt"
	}
	order := -1
	if c.Query("order") == "asc" {
		order = 1
	}
	sort := bson.D{{Key: sortBy, Value: order}}

	items, total, err := h.socialService.ListAccounts(c.Request.Context(), userOID, platformFilter, skip, limit, sort)
	if err != nil {
		c.Error(err)
		return
	}

	meta := utils.PaginationMeta{
		Page:  page,
		Limit: limit,
		Total: total,
	}

	utils.SendSuccess(c, http.StatusOK, "Social accounts fetched", items, meta)
}

func (h *SocialHandler) ConnectPlatform(c *gin.Context) {
	platform := strings.ToLower(c.Param("platform"))
	if platform == "instagram" {
		h.instagramHandler.Connect(c)
		return
	}
	if platform == "linkedin" {
		h.linkedinHandler.Connect(c)
		return
	}

	userVal, exists := c.Get("userId")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userID := userVal.(string)

	authURL, err := h.socialService.GetConnectURL(userID, platform)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "OAuth URL generated", gin.H{"authUrl": authURL})
}

func (h *SocialHandler) OAuthCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	platform := strings.ToLower(c.Param("platform"))

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, utils.APIResponse{
			Success: false,
			Message: "Missing code or state parameter",
		})
		return
	}

	userID, err := common.ValidateOAuthState(state, h.jwtAccessSecret)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{
			Success: false,
			Message: "Invalid or expired OAuth state",
		})
		return
	}

	socialHandle, err := h.socialService.ConnectFromCallback(c.Request.Context(), userID, platform, code, state)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Social account connected", socialHandle)
}

type RefreshTokenRequest struct {
	SocialID string `json:"socialId" binding:"required"`
}

func (h *SocialHandler) RefreshSocialToken(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	socialOID, err := bson.ObjectIDFromHex(req.SocialID)
	if err != nil {
		c.Error(utils.BadRequest("Invalid socialId format"))
		return
	}

	handle, err := h.socialService.RefreshPlatformToken(c.Request.Context(), userOID, socialOID)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Social token refreshed", handle)
}

func (h *SocialHandler) Disconnect(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	id := c.Param("id")
	socialOID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		c.Error(utils.BadRequest("Invalid social account ID"))
		return
	}

	if err := h.socialService.DisconnectAccount(c.Request.Context(), userOID, socialOID); err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Social account disconnected", nil)
}
