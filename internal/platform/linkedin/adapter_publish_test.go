package linkedin

import (
	"context"
	"testing"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/models"
)

func TestLinkedInPublishValidation_MissingCredentials(t *testing.T) {
	adapter := NewPublishAdapter(config.OAuthPlatformConfig{})

	post := &models.Post{
		Caption: "LinkedIn Post",
	}
	handle := &models.SocialHandle{
		Platform:            "linkedin",
		PlatformID:          "li_12345",
		PlatformAccessToken: "", // Missing
	}

	resp, err := adapter.Publish(context.Background(), post, handle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "failed" {
		t.Errorf("expected failed status, got %s", resp.Status)
	}
}

func TestLinkedInPublishValidation_EmptyCaptionAndMedia(t *testing.T) {
	adapter := NewPublishAdapter(config.OAuthPlatformConfig{})

	post := &models.Post{
		Caption: "",
		Media:   []models.MediaItem{},
	}
	handle := &models.SocialHandle{
		Platform:            "linkedin",
		PlatformID:          "li_12345",
		PlatformAccessToken: "token_abc",
	}

	resp, err := adapter.Publish(context.Background(), post, handle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "failed" {
		t.Errorf("expected failed status for empty content, got %s", resp.Status)
	}
}

func TestLinkedInResolveImages_Base64(t *testing.T) {
	adapter := NewPublishAdapter(config.OAuthPlatformConfig{})

	// 1x1 transparent PNG in base64
	validBase64 := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

	post := &models.Post{
		Caption:  "Carousel Post",
		PostType: "carousel",
		Media: []models.MediaItem{
			{Type: "image", Base64Data: validBase64, MimeType: "image/png"},
			{Type: "image", Base64Data: validBase64, MimeType: "image/png"},
		},
	}

	resolved := adapter.resolveImages(context.Background(), post)
	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved images for carousel, got %d", len(resolved))
	}
	if resolved[0].MimeType != "image/png" {
		t.Errorf("expected image/png, got %s", resolved[0].MimeType)
	}
}
