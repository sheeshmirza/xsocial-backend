package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type AdCampaign struct {
	ID                 bson.ObjectID `bson:"_id,omitempty" json:"id"`
	BrandID            bson.ObjectID `bson:"brandId" json:"brandId"`
	Platform           string        `bson:"platform" json:"platform"` // meta, google, tiktok
	ExternalCampaignID string        `bson:"externalCampaignId" json:"externalCampaignId"`
	Name               string        `bson:"name" json:"name"`
	Status             string        `bson:"status" json:"status"` // ACTIVE, PAUSED, COMPLETED
	Objective          string        `bson:"objective" json:"objective"`
	Budget             float64       `bson:"budget" json:"budget"`
	Spend              float64       `bson:"spend" json:"spend"`
	Impressions        int64         `bson:"impressions" json:"impressions"`
	Clicks             int64         `bson:"clicks" json:"clicks"`
	Conversions        int64         `bson:"conversions" json:"conversions"`
	CTR                float64       `bson:"ctr" json:"ctr"`
	CPC                float64       `bson:"cpc" json:"cpc"`
	CPM                float64       `bson:"cpm" json:"cpm"`
	ROAS               float64       `bson:"roas" json:"roas"`
	DateRange          string        `bson:"dateRange" json:"dateRange"`
	CreatedAt          time.Time     `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time     `bson:"updatedAt" json:"updatedAt"`
}

type AdSummary struct {
	TotalSpend       float64 `json:"totalSpend"`
	TotalImpressions int64   `json:"totalImpressions"`
	TotalClicks      int64   `json:"totalClicks"`
	TotalConversions int64   `json:"totalConversions"`
	AverageCTR       float64 `json:"averageCtr"`
	AverageCPC       float64 `json:"averageCpc"`
	AverageROAS      float64 `json:"averageRoas"`
}

type CreateAdCampaignRequest struct {
	Platform           string  `json:"platform"`
	ExternalCampaignID string  `json:"externalCampaignId"`
	Name               string  `json:"name"`
	Status             string  `json:"status"`
	Objective          string  `json:"objective"`
	Budget             float64 `json:"budget"`
	Spend              float64 `json:"spend"`
	Impressions        int64   `json:"impressions"`
	Clicks             int64   `json:"clicks"`
	Conversions        int64    `json:"conversions"`
	ConversionValue    *float64 `json:"conversionValue,omitempty"`
	ROAS               *float64 `json:"roas,omitempty"`
	DateRange          string   `json:"dateRange"`
}

type UpdateAdCampaignRequest struct {
	Name            *string  `json:"name"`
	Status          *string  `json:"status"`
	Objective       *string  `json:"objective"`
	Budget          *float64 `json:"budget"`
	Spend           *float64 `json:"spend"`
	Impressions     *int64   `json:"impressions"`
	Clicks          *int64   `json:"clicks"`
	Conversions     *int64   `json:"conversions"`
	ConversionValue *float64 `json:"conversionValue,omitempty"`
	ROAS            *float64 `json:"roas,omitempty"`
	DateRange       *string  `json:"dateRange"`
}
