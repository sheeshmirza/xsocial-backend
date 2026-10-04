package api

import (
	"net/http"
	"time"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/utils"
)

func (s *Server) signupUser(w http.ResponseWriter, r *http.Request) {
	var req models.SignupRequest
	if !decode(w, r, 0, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.authService.Signup(r.Context(), req.Email, req.Password)
	if err != nil {
		writeApiError(w, err)
		return
	}
	resp := map[string]any{
		"success": true,
		"message": "User registered successfully",
		"data":    res,
	}
	if res.EmailVerificationToken != nil {
		resp["verificationToken"] = *res.EmailVerificationToken
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) verifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		var req models.VerifyEmailRequest
		if decode(w, r, 0, &req) {
			token = req.Token
		}
	}
	req := models.VerifyEmailRequest{Token: token}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.authService.VerifyEmail(r.Context(), req.Token); err != nil {
		writeApiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Email verified successfully",
	})
}

func (s *Server) loginUser(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if !decode(w, r, 0, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ip := clientIP(r)
	if blocked, remaining := s.checkAuthRate(ip); blocked {
		writeError(w, http.StatusTooManyRequests, "too many failed attempts, please try again in "+remaining.Round(time.Second).String())
		return
	}
	res, err := s.authService.Login(r.Context(), req.Email, req.Password, r.UserAgent(), ip)
	if err != nil {
		s.recordAuthFailure(ip)
		writeApiError(w, err)
		return
	}
	s.recordAuthSuccess(ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"message":      "Logged in successfully",
		"data":         res,
		"token":        res.Tokens.AccessToken,
		"refreshToken": res.Tokens.RefreshToken,
	})
}

func (s *Server) refreshToken(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshTokenRequest
	if !decode(w, r, 0, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ip := clientIP(r)
	if blocked, remaining := s.checkAuthRate(ip); blocked {
		writeError(w, http.StatusTooManyRequests, "too many failed attempts, please try again in "+remaining.Round(time.Second).String())
		return
	}
	res, err := s.authService.Refresh(r.Context(), req.RefreshToken, r.UserAgent(), ip)
	if err != nil {
		s.recordAuthFailure(ip)
		writeApiError(w, err)
		return
	}
	s.recordAuthSuccess(ip)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"data":         res,
		"token":        res.Tokens.AccessToken,
		"refreshToken": res.Tokens.RefreshToken,
	})
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ForgotPasswordRequest
	if !decode(w, r, 0, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	token, _ := s.authService.ForgotPassword(r.Context(), req.Email)
	resp := map[string]any{
		"success": true,
		"message": "If the account exists, a password reset email has been sent",
	}
	if token != "" {
		resp["resetToken"] = token
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req models.ResetPasswordRequest
	if !decode(w, r, 0, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.authService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		writeApiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Password reset successfully",
	})
}

func (s *Server) logoutUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		_ = decode(w, r, 0, &req)
	}
	if req.RefreshToken != "" {
		_ = s.authService.Logout(r.Context(), req.RefreshToken)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Logged out successfully",
	})
}

func (s *Server) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	user, err := s.userService.GetUserByID(r.Context(), userOID)
	if err != nil || user == nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    user.ToSanitized(),
	})
}

func (s *Server) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req models.UpdateProfileRequest
	if !decode(w, r, 0, &req) {
		return
	}
	req.FullName = utils.SanitizeString(req.FullName)
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.userService.UpdateProfile(r.Context(), userOID, req.FullName, req.Timezone)
	if err != nil {
		writeApiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Profile updated successfully",
		"data":    updated.ToSanitized(),
	})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req models.ChangePasswordRequest
	if !decode(w, r, 0, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.userService.ChangePassword(r.Context(), userOID, req.OldPassword, req.NewPassword); err != nil {
		writeApiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Password changed successfully",
	})
}

func (s *Server) deleteCurrentUser(w http.ResponseWriter, r *http.Request) {
	userOID, ok := currentUserOID(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if err := s.userService.DeleteUser(r.Context(), userOID); err != nil {
		writeApiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Account deleted successfully",
	})
}
