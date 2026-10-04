package models

import (
	"fmt"
	"strings"
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

func (r *CreateBrandRequest) Validate() error {
	r.Name = strings.ReplaceAll(r.Name, "\x00", "")
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return fmt.Errorf("brand name is required")
	}
	if len(r.Name) > 100 {
		return fmt.Errorf("brand name must not exceed 100 characters")
	}
	r.Description = strings.ReplaceAll(r.Description, "\x00", "")
	if len(r.Description) > 2000 {
		return fmt.Errorf("description must not exceed 2000 characters")
	}
	if r.LogoURL != "" {
		trimmed := strings.TrimSpace(r.LogoURL)
		if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			return fmt.Errorf("logoUrl must start with http:// or https://")
		}
	}
	if r.WebsiteURL != "" {
		trimmed := strings.TrimSpace(r.WebsiteURL)
		if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			return fmt.Errorf("websiteUrl must start with http:// or https://")
		}
	}
	if r.Currency != "" {
		r.Currency = strings.ToUpper(strings.TrimSpace(r.Currency))
		if len(r.Currency) != 3 {
			return fmt.Errorf("currency must be a 3-letter ISO code")
		}
	}
	if r.Timezone != "" {
		if _, err := time.LoadLocation(r.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", r.Timezone)
		}
	}
	return nil
}

type UpdateBrandRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	LogoURL     *string `json:"logoUrl"`
	Timezone    *string `json:"timezone"`
	Currency    *string `json:"currency"`
	WebsiteURL  *string `json:"websiteUrl"`
}

func (r *UpdateBrandRequest) Validate() error {
	if r.Name != nil {
		*r.Name = strings.ReplaceAll(*r.Name, "\x00", "")
		*r.Name = strings.TrimSpace(*r.Name)
		if *r.Name == "" {
			return fmt.Errorf("brand name cannot be empty")
		}
		if len(*r.Name) > 100 {
			return fmt.Errorf("brand name must not exceed 100 characters")
		}
	}
	if r.Description != nil {
		*r.Description = strings.ReplaceAll(*r.Description, "\x00", "")
		if len(*r.Description) > 2000 {
			return fmt.Errorf("description must not exceed 2000 characters")
		}
	}
	if r.LogoURL != nil && *r.LogoURL != "" {
		trimmed := strings.TrimSpace(*r.LogoURL)
		if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			return fmt.Errorf("logoUrl must start with http:// or https://")
		}
	}
	if r.WebsiteURL != nil && *r.WebsiteURL != "" {
		trimmed := strings.TrimSpace(*r.WebsiteURL)
		if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
			return fmt.Errorf("websiteUrl must start with http:// or https://")
		}
	}
	if r.Currency != nil && *r.Currency != "" {
		*r.Currency = strings.ToUpper(strings.TrimSpace(*r.Currency))
		if len(*r.Currency) != 3 {
			return fmt.Errorf("currency must be a 3-letter ISO code")
		}
	}
	if r.Timezone != nil && *r.Timezone != "" {
		if _, err := time.LoadLocation(*r.Timezone); err != nil {
			return fmt.Errorf("invalid timezone: %s", *r.Timezone)
		}
	}
	return nil
}

type AddBrandMemberRequest struct {
	Email string    `json:"email" binding:"required,email"`
	Name  string    `json:"name"`
	Role  BrandRole `json:"role" binding:"required"`
}

func (r *AddBrandMemberRequest) Validate() error {
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	if r.Email == "" {
		return fmt.Errorf("email is required")
	}
	if len(r.Email) < 3 || len(r.Email) > 254 || !strings.Contains(r.Email, "@") || !strings.Contains(r.Email, ".") {
		return fmt.Errorf("invalid email address format")
	}
	r.Name = strings.TrimSpace(r.Name)
	if len(r.Name) > 100 {
		return fmt.Errorf("name must not exceed 100 characters")
	}
	validRoles := map[BrandRole]bool{
		BrandRoleOwner: true, BrandRoleAdmin: true, BrandRoleManager: true,
		BrandRoleEditor: true, BrandRoleApprover: true, BrandRoleViewer: true,
		BrandRoleClient: true,
	}
	if !validRoles[r.Role] {
		return fmt.Errorf("invalid brand role: %s", r.Role)
	}
	return nil
}
