package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type User struct {
	ID                         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email                      string        `bson:"email" json:"email"`
	Password                   string        `bson:"password" json:"-"`
	FullName                   string        `bson:"fullName" json:"fullName"`
	IsVerified                 bool          `bson:"isVerified" json:"isVerified"`
	EmailVerificationToken     *string       `bson:"emailVerificationToken,omitempty" json:"-"`
	EmailVerificationExpiresAt *time.Time    `bson:"emailVerificationExpiresAt,omitempty" json:"-"`
	ResetPasswordToken         *string       `bson:"resetPasswordToken,omitempty" json:"-"`
	ResetPasswordExpiresAt     *time.Time    `bson:"resetPasswordExpiresAt,omitempty" json:"-"`
	Timezone                   string        `bson:"timezone" json:"timezone"`
	LastLoginAt                *time.Time    `bson:"lastLoginAt,omitempty" json:"lastLoginAt,omitempty"`
	CreatedAt                  time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt                  time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type UserProfileResponse struct {
	ID         bson.ObjectID `json:"id"`
	Email      string        `json:"email"`
	IsVerified bool          `json:"isVerified"`
	FullName   string        `json:"fullName"`
	Timezone   string        `json:"timezone"`
	CreatedAt  time.Time     `json:"createdAt"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}

func (u *User) ToProfileResponse() UserProfileResponse {
	return UserProfileResponse{
		ID:         u.ID,
		Email:      u.Email,
		IsVerified: u.IsVerified,
		FullName:   u.FullName,
		Timezone:   u.Timezone,
		CreatedAt:  u.CreatedAt,
		UpdatedAt:  u.UpdatedAt,
	}
}

type UserSanitized struct {
	ID         bson.ObjectID `json:"id"`
	Email      string        `json:"email"`
	FullName   string        `json:"fullName"`
	Timezone   string        `json:"timezone"`
	IsVerified bool          `json:"isVerified"`
	CreatedAt  time.Time     `json:"createdAt"`
}

func (u *User) ToSanitized() UserSanitized {
	return UserSanitized{
		ID:         u.ID,
		Email:      u.Email,
		FullName:   u.FullName,
		Timezone:   u.Timezone,
		IsVerified: u.IsVerified,
		CreatedAt:  u.CreatedAt,
	}
}

const (
	PasswordMinLen = 8
	PasswordMaxLen = 128
)

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *SignupRequest) Validate() error {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(r.Email) < 3 || len(r.Email) > 254 || strings.Contains(r.Email, "..") || !emailRegex.MatchString(r.Email) {
		return fmt.Errorf("invalid email address format")
	}
	if len(r.Password) < PasswordMinLen {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(r.Password) > PasswordMaxLen {
		return fmt.Errorf("password must not exceed 128 characters")
	}
	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(r.Email) < 3 || len(r.Email) > 254 || strings.Contains(r.Email, "..") || !emailRegex.MatchString(r.Email) {
		return fmt.Errorf("invalid email address format")
	}
	if r.Password == "" {
		return fmt.Errorf("password is required")
	}
	if len(r.Password) > PasswordMaxLen {
		return fmt.Errorf("password must not exceed 128 characters")
	}
	return nil
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (r *RefreshTokenRequest) Validate() error {
	r.RefreshToken = strings.TrimSpace(r.RefreshToken)
	if r.RefreshToken == "" {
		return fmt.Errorf("refreshToken is required")
	}
	if len(r.RefreshToken) > 1024 {
		return fmt.Errorf("refreshToken exceeds maximum allowed length")
	}
	return nil
}

type VerifyEmailRequest struct {
	Token string `json:"token"`
}

func (r *VerifyEmailRequest) Validate() error {
	r.Token = strings.TrimSpace(r.Token)
	if r.Token == "" {
		return fmt.Errorf("verification token is required")
	}
	if len(r.Token) > 512 {
		return fmt.Errorf("verification token exceeds maximum allowed length")
	}
	return nil
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

func (r *ForgotPasswordRequest) Validate() error {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(r.Email) < 3 || len(r.Email) > 254 || strings.Contains(r.Email, "..") || !emailRegex.MatchString(r.Email) {
		return fmt.Errorf("invalid email address format")
	}
	return nil
}

type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func (r *ResetPasswordRequest) Validate() error {
	r.Token = strings.TrimSpace(r.Token)
	if r.Token == "" {
		return fmt.Errorf("reset token is required")
	}
	if len(r.Token) > 512 {
		return fmt.Errorf("reset token exceeds maximum allowed length")
	}
	if len(r.NewPassword) < PasswordMinLen {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(r.NewPassword) > PasswordMaxLen {
		return fmt.Errorf("password must not exceed 128 characters")
	}
	return nil
}

type UpdateProfileRequest struct {
	FullName string `json:"fullName"`
	Timezone string `json:"timezone"`
}

func (r *UpdateProfileRequest) Validate() error {
	r.FullName = strings.ReplaceAll(r.FullName, "\x00", "")
	r.FullName = strings.TrimSpace(r.FullName)
	if len(r.FullName) > 100 {
		return fmt.Errorf("fullName must not exceed 100 characters")
	}
	r.Timezone = strings.TrimSpace(r.Timezone)
	if r.Timezone != "" && r.Timezone != "UTC" {
		if _, err := time.LoadLocation(r.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", r.Timezone)
		}
	}
	return nil
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (r *ChangePasswordRequest) Validate() error {
	if r.OldPassword == "" {
		return fmt.Errorf("oldPassword is required")
	}
	if len(r.NewPassword) < PasswordMinLen {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(r.NewPassword) > PasswordMaxLen {
		return fmt.Errorf("password must not exceed 128 characters")
	}
	return nil
}
