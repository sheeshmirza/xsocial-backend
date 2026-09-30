package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"xsocial-backend/internal/service"
	"xsocial-backend/internal/utils"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

type SignupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

func (h *AuthHandler) Signup(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	user, err := h.authService.Signup(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusCreated, "Signup successful. Verify your email.", user.ToSanitized())
}

type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	var req VerifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	if err := h.authService.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Email verified successfully", nil)
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	res, err := h.authService.Login(c.Request.Context(), req.Email, req.Password, userAgent, ip)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Login successful", res)
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	userAgent := c.GetHeader("User-Agent")
	ip := c.ClientIP()

	res, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken, userAgent, ip)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Token refreshed", res)
}

type LogoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	_ = c.ShouldBindJSON(&req)

	_ = h.authService.Logout(c.Request.Context(), req.RefreshToken)
	utils.SendSuccess(c, http.StatusOK, "Logout successful", nil)
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	_ = h.authService.ForgotPassword(c.Request.Context(), req.Email)
	utils.SendSuccess(c, http.StatusOK, "If the email exists, reset instructions were sent", nil)
}

type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.BadRequest("Validation failed", err.Error()))
		return
	}

	if err := h.authService.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		c.Error(err)
		return
	}

	utils.SendSuccess(c, http.StatusOK, "Password reset successful", nil)
}
