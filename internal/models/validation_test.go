package models_test

import (
	"strings"
	"testing"

	"xsocial-backend/internal/models"
)

func TestCreatePostInput_Validate(t *testing.T) {
	t.Run("Valid Post Input", func(t *testing.T) {
		in := models.CreatePostInput{
			Title:      "My First Post",
			Caption:    "Hello world!",
			PostType:   "mixed",
			Visibility: "public",
			Timezone:   "UTC",
			Recurrence: models.RecurrenceOnce,
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid input, got: %v", err)
		}
	})

	t.Run("Title Too Long", func(t *testing.T) {
		in := models.CreatePostInput{
			Title: strings.Repeat("A", 501),
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for title exceeding 500 chars")
		}
	})

	t.Run("Invalid Post Type", func(t *testing.T) {
		in := models.CreatePostInput{
			PostType: "malicious_type",
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for invalid post type")
		}
	})

	t.Run("Invalid Timezone", func(t *testing.T) {
		in := models.CreatePostInput{
			Timezone: "Fantasy/Narnia",
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for invalid timezone")
		}
	})

	t.Run("Invalid Media URL Scheme", func(t *testing.T) {
		in := models.CreatePostInput{
			Media: []models.MediaItem{
				{Type: "image", URL: "ftp://unsafe-server/img.jpg"},
			},
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for ftp:// scheme")
		}
	})
}

func TestCreateBrandRequest_Validate(t *testing.T) {
	t.Run("Valid Brand Request", func(t *testing.T) {
		req := models.CreateBrandRequest{
			Name:     "Acme Corp",
			Currency: "USD",
			Timezone: "America/New_York",
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Brand Name", func(t *testing.T) {
		req := models.CreateBrandRequest{
			Name: "   ",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty brand name")
		}
	})

	t.Run("Invalid Currency Code", func(t *testing.T) {
		req := models.CreateBrandRequest{
			Name:     "Brand",
			Currency: "INVALID",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid currency")
		}
	})
}

func TestCreateApiKeyRequest_Validate(t *testing.T) {
	t.Run("Valid Api Key Request", func(t *testing.T) {
		req := models.CreateApiKeyRequest{
			Name:          "Production Worker",
			ExpiresInDays: 30,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Empty Key Name", func(t *testing.T) {
		req := models.CreateApiKeyRequest{
			Name: "",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty key name")
		}
	})

	t.Run("Negative Expiration", func(t *testing.T) {
		req := models.CreateApiKeyRequest{
			Name:          "Key",
			ExpiresInDays: -10,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for negative expiresInDays")
		}
	})
}
