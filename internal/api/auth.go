package api

import (
	"net/http"
	"strings"
)

func (s *Server) signupUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	res, err := s.authService.Signup(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"message": "User registered successfully",
		"data":    res,
	})
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
		writeError(w, http.StatusBadRequest, err.Error())
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

	res, err := s.authService.Login(r.Context(), req.Email, req.Password, r.UserAgent(), clientIP(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Logged in successfully",
		"data":    res,
	})
}

func (s *Server) refreshToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	res, err := s.authService.Refresh(r.Context(), req.RefreshToken, r.UserAgent(), clientIP(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    res,
	})
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	_ = s.authService.ForgotPassword(r.Context(), req.Email)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "If the account exists, a password reset email has been sent",
	})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if !decode(w, r, 0, &req) {
		return
	}

	if err := s.authService.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Account deleted successfully",
	})
}
