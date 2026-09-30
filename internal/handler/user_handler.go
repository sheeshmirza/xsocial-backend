package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) GetMe(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	user := userVal.(*models.User)
	utils.SendSuccess(c, http.StatusOK, "Profile fetched", user.ToProfileResponse())
}

type UpdateProfileRequest struct {
	FullName string `json:"fullName"`
	Timezone string `json:"timezone"`
}

func (h *UserHandler) UpdateMe(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	updated, err := h.userService.UpdateProfile(c.Request.Context(), userOID, req.FullName, req.Timezone)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Profile updated", updated.ToProfileResponse())
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	_, err := h.userService.ChangePassword(c.Request.Context(), userOID, req.OldPassword, req.NewPassword)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Password changed successfully", nil)
}

func (h *UserHandler) DeleteMe(c *gin.Context) {
	userOIDVal, exists := c.Get("userOID")
	if !exists {
		c.Error(utils.Unauthorized("Unauthorized"))
		return
	}
	userOID := userOIDVal.(bson.ObjectID)

	if err := h.userService.DeleteUser(c.Request.Context(), userOID); err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Account deleted successfully", nil)
}
