package instagram

import (
	"context"
	"testing"

	"xsocial-backend/internal/models"
)

func TestInstagramPublishValidation_MissingCredentials(t *testing.T) {
	adapter := NewPublishAdapter()

	post := &models.Post{
		Caption: "Test Caption",
		Media: []models.MediaItem{
			{Type: "image", URL: "https://example.com/test.jpg"},
		},
	}
	handle := &models.SocialHandle{
		Platform:   "instagram",
		PlatformID: "", // Missing
	}

	resp, err := adapter.Publish(context.Background(), post, handle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "failed" {
		t.Errorf("expected failed status, got %s", resp.Status)
	}
}

func TestInstagramPublishValidation_MissingMedia(t *testing.T) {
	adapter := NewPublishAdapter()

	post := &models.Post{
		Caption: "Text-only post without media",
		Media:   []models.MediaItem{},
	}
	handle := &models.SocialHandle{
		Platform:            "instagram",
		PlatformID:          "ig_12345",
		PlatformAccessToken: "token_abc",
	}

	resp, err := adapter.Publish(context.Background(), post, handle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "failed" {
		t.Errorf("expected failed status for Instagram without media, got %s", resp.Status)
	}
}

func TestInstagramPublishValidation_InaccessibleURL(t *testing.T) {
	adapter := NewPublishAdapter()

	post := &models.Post{
		Caption: "Memory URL",
		Media: []models.MediaItem{
			{Type: "image", URL: "memory://buffer-123"},
		},
	}
	handle := &models.SocialHandle{
		Platform:            "instagram",
		PlatformID:          "ig_12345",
		PlatformAccessToken: "token_abc",
	}

	resp, err := adapter.Publish(context.Background(), post, handle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "failed" {
		t.Errorf("expected failed status for inaccessible media, got %s", resp.Status)
	}
}

func TestInstagramTrialReelSettings(t *testing.T) {
	post := &models.Post{
		Caption:  "Trial Reel",
		PostType: "reel",
		Media: []models.MediaItem{
			{Type: "video", URL: "https://example.com/test.mp4"},
		},
		InstagramTrial: &models.InstagramTrialSettings{
			Enabled:            true,
			GraduationStrategy: "MANUAL",
		},
	}

	if post.InstagramTrial == nil || !post.InstagramTrial.Enabled {
		t.Fatalf("expected InstagramTrial to be enabled")
	}
	if post.InstagramTrial.GraduationStrategy != "MANUAL" {
		t.Errorf("expected MANUAL graduation strategy, got %s", post.InstagramTrial.GraduationStrategy)
	}
}

