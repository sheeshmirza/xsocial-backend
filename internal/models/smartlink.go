package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type SmartLinkButtonType string

const (
	ButtonTypeLink       SmartLinkButtonType = "link"
	ButtonTypeBanner     SmartLinkButtonType = "banner"
	ButtonTypeVideo      SmartLinkButtonType = "video"
	ButtonTypeProduct    SmartLinkButtonType = "product"
	ButtonTypeSocialGrid SmartLinkButtonType = "social_grid"
)

type SmartLinkButton struct {
	ID        string              `bson:"id" json:"id"`
	Title     string              `bson:"title" json:"title"`
	URL       string              `bson:"url" json:"url"`
	Icon      string              `bson:"icon" json:"icon"`
	ImageURL  string              `bson:"imageUrl" json:"imageUrl"`
	Type      SmartLinkButtonType `bson:"type" json:"type"`
	Price     string              `bson:"price" json:"price"`
	IsActive  bool                `bson:"isActive" json:"isActive"`
	Order     int                 `bson:"order" json:"order"`
	Clicks    int64               `bson:"clicks" json:"clicks"`
	StartDate *time.Time          `bson:"startDate,omitempty" json:"startDate,omitempty"`
	EndDate   *time.Time          `bson:"endDate,omitempty" json:"endDate,omitempty"`
}

type SmartLinkPage struct {
	ID           bson.ObjectID       `bson:"_id,omitempty" json:"id"`
	BrandID      bson.ObjectID       `bson:"brandId" json:"brandId"`
	Slug         string              `bson:"slug" json:"slug"`
	Title        string              `bson:"title" json:"title"`
	Bio          string              `bson:"bio" json:"bio"`
	AvatarURL    string              `bson:"avatarUrl" json:"avatarUrl"`
	Theme        string              `bson:"theme" json:"theme"` // light, dark, gradient, custom
	CustomColors map[string]string   `bson:"customColors" json:"customColors"`
	SocialLinks  map[string]string   `bson:"socialLinks" json:"socialLinks"`
	Buttons      []SmartLinkButton   `bson:"buttons" json:"buttons"`
	TotalViews   int64               `bson:"totalViews" json:"totalViews"`
	TotalClicks  int64               `bson:"totalClicks" json:"totalClicks"`
	IsActive     bool                `bson:"isActive" json:"isActive"`
	CreatedAt    time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time           `bson:"updatedAt" json:"updatedAt"`
}

type CreateSmartLinkRequest struct {
	Slug         string            `json:"slug" binding:"required"`
	Title        string            `json:"title" binding:"required"`
	Bio          string            `json:"bio"`
	AvatarURL    string            `json:"avatarUrl"`
	Theme        string            `json:"theme"`
	CustomColors map[string]string `json:"customColors"`
	SocialLinks  map[string]string `json:"socialLinks"`
	Buttons      []SmartLinkButton `json:"buttons"`
}

type UpdateSmartLinkRequest struct {
	Title        *string            `json:"title"`
	Bio          *string            `json:"bio"`
	AvatarURL    *string            `json:"avatarUrl"`
	Theme        *string            `json:"theme"`
	CustomColors map[string]string  `json:"customColors"`
	SocialLinks  map[string]string  `json:"socialLinks"`
	Buttons      *[]SmartLinkButton `json:"buttons"`
	IsActive     *bool              `json:"isActive"`
}
