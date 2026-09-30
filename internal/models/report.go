package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "pending"
	ReportStatusCompleted ReportStatus = "completed"
	ReportStatusFailed    ReportStatus = "failed"
)

type Report struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"id"`
	BrandID        bson.ObjectID `bson:"brandId" json:"brandId"`
	Title          string        `bson:"title" json:"title"`
	PeriodStart    time.Time     `bson:"periodStart" json:"periodStart"`
	PeriodEnd      time.Time     `bson:"periodEnd" json:"periodEnd"`
	Format         string        `bson:"format" json:"format"` // pdf, csv, json
	FileURL        string        `bson:"fileUrl" json:"fileUrl"`
	Status         ReportStatus  `bson:"status" json:"status"`
	WhiteLabelLogo string         `bson:"whiteLabelLogo" json:"whiteLabelLogo"`
	Metrics        map[string]any `bson:"metrics,omitempty" json:"metrics,omitempty"`
	CreatedAt      time.Time      `bson:"createdAt" json:"createdAt"`
	CompletedAt    *time.Time     `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
}

type GenerateReportRequest struct {
	BrandID        string `json:"brandId" binding:"required"`
	Title          string `json:"title" binding:"required"`
	PeriodStart    string `json:"periodStart" binding:"required"` // ISO timestamp
	PeriodEnd      string `json:"periodEnd" binding:"required"`   // ISO timestamp
	Format         string `json:"format"`                         // pdf, csv, json
	WhiteLabelLogo string `json:"whiteLabelLogo"`
}
