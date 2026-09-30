package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

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
