package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type BrandRole string

const (
	BrandRoleOwner    BrandRole = "owner"
	BrandRoleAdmin    BrandRole = "admin"
	BrandRoleManager  BrandRole = "manager"
	BrandRoleEditor   BrandRole = "editor"
	BrandRoleApprover BrandRole = "approver"
	BrandRoleViewer   BrandRole = "viewer"
	BrandRoleClient   BrandRole = "client"
)

type Brand struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerID     bson.ObjectID `bson:"ownerId" json:"ownerId"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description" json:"description"`
	LogoURL     string        `bson:"logoUrl" json:"logoUrl"`
	Timezone    string        `bson:"timezone" json:"timezone"`
	Currency    string        `bson:"currency" json:"currency"`
	WebsiteURL  string        `bson:"websiteUrl" json:"websiteUrl"`
	CreatedAt   time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type BrandMember struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	BrandID   bson.ObjectID `bson:"brandId" json:"brandId"`
	UserID    bson.ObjectID `bson:"userId" json:"userId"`
	Email     string        `bson:"email" json:"email"`
	Name      string        `bson:"name" json:"name"`
	Role      BrandRole     `bson:"role" json:"role"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type CreateBrandRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	LogoURL     string `json:"logoUrl"`
	Timezone    string `json:"timezone"`
	Currency    string `json:"currency"`
	WebsiteURL  string `json:"websiteUrl"`
}

type UpdateBrandRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	LogoURL     *string `json:"logoUrl"`
	Timezone    *string `json:"timezone"`
	Currency    *string `json:"currency"`
	WebsiteURL  *string `json:"websiteUrl"`
}

type AddBrandMemberRequest struct {
	Email string    `json:"email" binding:"required,email"`
	Name  string    `json:"name"`
	Role  BrandRole `json:"role" binding:"required"`
}
