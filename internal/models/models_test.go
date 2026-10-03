package models_test

import (
	"testing"
	"time"

	"xsocial-backend/internal/models"
)

func TestPostRecurrenceConfig(t *testing.T) {
	t.Run("Standard naming recurrence", func(t *testing.T) {
		p := &models.Post{Recurrence: models.RecurrenceDaily}
		rec := p.GetRecurrenceConfig()
		if rec == nil || rec.Interval != 1 || rec.Unit != "day" {
			t.Errorf("expected daily recurrence interval=1, unit=day; got %+v", rec)
		}
	})

	t.Run("Custom recurrence interval and unit", func(t *testing.T) {
		interval := 3
		unit := "week"
		p := &models.Post{
			RecurrenceInterval: &interval,
			RecurrenceUnit:     &unit,
		}
		rec := p.GetRecurrenceConfig()
		if rec == nil || rec.Interval != 3 || rec.Unit != "week" {
			t.Errorf("expected interval=3, unit=week; got %+v", rec)
		}
	})

	t.Run("Invalid custom recurrence falls back", func(t *testing.T) {
		unit := "minute"
		invalidInterval := -5
		p := &models.Post{
			Recurrence:         models.RecurrenceOnce,
			RecurrenceInterval: &invalidInterval,
			RecurrenceUnit:     &unit,
		}
		rec := p.GetRecurrenceConfig()
		if rec != nil {
			t.Errorf("expected nil recurrence for once/invalid; got %+v", rec)
		}
	})
}

func TestIsValidRecurrenceInterval(t *testing.T) {
	tests := []struct {
		interval int
		unit     string
		expected bool
	}{
		{30, "minute", true},
		{29, "minute", false},
		{1, "hour", true},
		{8760, "hour", true},
		{8761, "hour", false},
		{1, "day", true},
		{365, "day", true},
		{366, "day", false},
		{1, "week", true},
		{52, "week", true},
		{53, "week", false},
		{1, "month", true},
		{12, "month", true},
		{13, "month", false},
		{1, "invalid_unit", false},
	}

	for _, tt := range tests {
		result := models.IsValidRecurrenceInterval(tt.interval, tt.unit)
		if result != tt.expected {
			t.Errorf("IsValidRecurrenceInterval(%d, %q) = %v; want %v", tt.interval, tt.unit, result, tt.expected)
		}
	}
}

func TestPostStatusHelpers(t *testing.T) {
	now := time.Now()
	p := &models.Post{
		Status:        models.PostStatusPublished,
		PublishedTime: &now,
	}

	if !p.IsPublished() {
		t.Error("expected IsPublished to be true")
	}
	if p.IsDraft() {
		t.Error("expected IsDraft to be false")
	}
	if p.IsScheduled() {
		t.Error("expected IsScheduled to be false")
	}
}

func TestNormalizeAccountTypeByPlatform(t *testing.T) {
	if res := models.NormalizeAccountTypeByPlatform("instagram", "business"); res != models.InstagramAccountTypeBusiness {
		t.Errorf("expected %q, got %q", models.InstagramAccountTypeBusiness, res)
	}
	if res := models.NormalizeAccountTypeByPlatform("instagram", "other"); res != models.InstagramAccountTypeCreator {
		t.Errorf("expected %q, got %q", models.InstagramAccountTypeCreator, res)
	}
	if res := models.NormalizeAccountTypeByPlatform("linkedin", "organization"); res != models.LinkedInAccountTypeOrganization {
		t.Errorf("expected %q, got %q", models.LinkedInAccountTypeOrganization, res)
	}
	if res := models.NormalizeAccountTypeByPlatform("linkedin", "other"); res != models.LinkedInAccountTypePerson {
		t.Errorf("expected %q, got %q", models.LinkedInAccountTypePerson, res)
	}
}
