package instagram

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"xsocial-backend/internal/platform/common"
	"xsocial-backend/internal/utils"
)

type Handler struct {
	service         *Service
	jwtAccessSecret string
}

func NewHandler(service *Service, jwtAccessSecret string) *Handler {
	return &Handler{
		service:         service,
		jwtAccessSecret: jwtAccessSecret,
	}
}

func (h *Handler) Connect(c *gin.Context) {
	userVal, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, utils.APIResponse{Success: false, Message: "Unauthorized"})
		return
	}
	userID := userVal.(string)

	state := common.GenerateOAuthState(userID, h.jwtAccessSecret)
	authURL := h.service.GetConnectURL(state)

	utils.SendSuccess(c, http.StatusOK, "OAuth URL generated", gin.H{"authUrl": authURL})
}

func (h *Handler) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")

	if code == "" || state == "" {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Missing code or state parameter"})
		return
	}

	_, err := common.ValidateOAuthState(state, h.jwtAccessSecret)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: "Invalid or expired OAuth state"})
		return
	}

	handle, err := h.service.HandleCallback(c.Request.Context(), code, state)
	if err != nil {
		c.JSON(http.StatusBadRequest, utils.APIResponse{Success: false, Message: err.Error()})
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Instagram account connected", handle)
}
