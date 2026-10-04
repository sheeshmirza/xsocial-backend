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

func TestUpdatePostInput_Validate(t *testing.T) {
	t.Run("Valid Update Input", func(t *testing.T) {
		title := "Updated title"
		postType := "reel"
		visibility := "public"
		in := models.UpdatePostInput{
			Title:      &title,
			PostType:   &postType,
			Visibility: &visibility,
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid update input, got: %v", err)
		}
	})

	t.Run("Invalid Post Type", func(t *testing.T) {
		badType := "unsupported_type"
		in := models.UpdatePostInput{
			PostType: &badType,
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for unsupported post type")
		}
	})

	t.Run("Title Too Long", func(t *testing.T) {
		longTitle := strings.Repeat("A", 501)
		in := models.UpdatePostInput{
			Title: &longTitle,
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for title exceeding 500 characters")
		}
	})
}

func TestBulkIDsInput_Validate(t *testing.T) {
	t.Run("Valid IDs", func(t *testing.T) {
		in := models.BulkIDsInput{
			IDs: []string{"id1", "id2"},
		}
		if err := in.Validate(); err != nil {
			t.Errorf("expected valid bulk input, got: %v", err)
		}
	})

	t.Run("Empty IDs", func(t *testing.T) {
		in := models.BulkIDsInput{
			IDs: []string{},
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for empty IDs")
		}
	})

	t.Run("Exceeds Limit", func(t *testing.T) {
		ids := make([]string, 101)
		for i := range ids {
			ids[i] = "id"
		}
		in := models.BulkIDsInput{
			IDs: ids,
		}
		if err := in.Validate(); err == nil {
			t.Error("expected error for exceeding 100 IDs")
		}
	})
}

func TestResharePostRequest_Validate(t *testing.T) {
	t.Run("Valid Reshare Request", func(t *testing.T) {
		caption := "Great post!"
		req := models.ResharePostRequest{
			TargetPlatforms:    []string{"linkedin"},
			NewCaption:         &caption,
			PublishImmediately: true,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid reshare request, got: %v", err)
		}
	})

	t.Run("Missing Target Accounts and Platforms", func(t *testing.T) {
		req := models.ResharePostRequest{
			PublishImmediately: true,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for missing targets")
		}
	})
}

func TestUpdateBrandRequest_Validate(t *testing.T) {
	t.Run("Valid Update", func(t *testing.T) {
		name := "New Brand Name"
		curr := "EUR"
		req := models.UpdateBrandRequest{
			Name:     &name,
			Currency: &curr,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid update, got: %v", err)
		}
	})

	t.Run("Empty Name", func(t *testing.T) {
		name := "   "
		req := models.UpdateBrandRequest{
			Name: &name,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for empty name")
		}
	})

	t.Run("Invalid Currency", func(t *testing.T) {
		curr := "US"
		req := models.UpdateBrandRequest{
			Currency: &curr,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid currency length")
		}
	})
}

func TestAddBrandMemberRequest_Validate(t *testing.T) {
	t.Run("Valid Member Request", func(t *testing.T) {
		req := models.AddBrandMemberRequest{
			Email: "member@example.com",
			Name:  "Jane Doe",
			Role:  models.BrandRoleEditor,
		}
		if err := req.Validate(); err != nil {
			t.Errorf("expected valid, got: %v", err)
		}
	})

	t.Run("Invalid Email", func(t *testing.T) {
		req := models.AddBrandMemberRequest{
			Email: "invalid-email",
			Role:  models.BrandRoleViewer,
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid email")
		}
	})

	t.Run("Invalid Role", func(t *testing.T) {
		req := models.AddBrandMemberRequest{
			Email: "test@example.com",
			Role:  "superadmin",
		}
		if err := req.Validate(); err == nil {
			t.Error("expected error for invalid role")
		}
	})
}

