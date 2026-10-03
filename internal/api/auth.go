package api

import (
	"net/http"
	"regexp"
	"strings"
)

// emailRegexp is a minimal RFC 5321 email validator.
var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

const (
	passwordMinLen = 8
	passwordMaxLen = 128
)

func (s *Server) signupUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, 0, &req) {
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if !emailRegexp.MatchString(req.Email) {
		writeError(w, http.StatusBadRequest, "invalid email address format")
		return
	}
	if len(req.Password) < passwordMinLen {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if len(req.Password) > passwordMaxLen {
		writeError(w, http.StatusBadRequest, "password must not exceed 128 characters")
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
		var req struct {
			Token string `json:"token"`
		}
		if decode(w, r, 0, &req) {
			token = req.Token
		}
	}
	if token == "" {
		writeError(w, http.StatusBadRequest, "verification token is required")
		return
	}
	if err := s.authService.VerifyEmail(r.Context(), token); err != nil {
		writeApiError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Email verified successfully",
	})
}

func (s *Server) loginUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, 0, &req) {
		return
	}
	ip := clientIP(r)
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
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decode(w, r, 0, &req) {
		return
	}
	ip := clientIP(r)
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
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, 0, &req) {
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
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if !decode(w, r, 0, &req) {
		return
	}
	if req.Token == "" {
		writeError(w, http.StatusBadRequest, "reset token is required")
		return
	}
	if len(req.NewPassword) < passwordMinLen {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}
	if len(req.NewPassword) > passwordMaxLen {
		writeError(w, http.StatusBadRequest, "password must not exceed 128 characters")
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
	var req struct {
		FullName string `json:"fullName"`
		Timezone string `json:"timezone"`
	}
	if !decode(w, r, 0, &req) {
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
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !decode(w, r, 0, &req) {
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
