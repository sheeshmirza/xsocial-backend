package service_test

import (
	"context"
	"mime/multipart"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/service"
)

func TestUploadService_NilConfigFaultTolerance(t *testing.T) {
	svc := service.NewUploadService(nil)

	// Save and retrieve media
	id := "test-media-id"
	buf := []byte("image-data")
	svc.SaveMedia(id, buf, "image/png")

	media := svc.GetMedia(id)
	if media == nil {
		t.Fatal("expected media to be retrieved")
	}
	if string(media.Buffer) != "image-data" {
		t.Errorf("expected buffer %q, got %q", "image-data", string(media.Buffer))
	}

	// Missing media returns nil safely
	if missing := svc.GetMedia("nonexistent"); missing != nil {
		t.Errorf("expected nil for nonexistent media, got %v", missing)
	}

	// Empty upload files returns error safely without nil pointer panic
	_, err := svc.ProcessUploads([]*multipart.FileHeader{}, "localhost:5000")
	if err == nil {
		t.Error("expected error for empty upload list")
	}
}

func TestAuditLogService_NilRepoFaultTolerance(t *testing.T) {
	svc := service.NewAuditLogService(nil)

	// Calling Log with nil repo must not panic
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic in Log with nil repo: %v", r)
		}
	}()

	userOID := bson.NewObjectID()
	svc.Log(context.Background(), &userOID, nil, "system", "test.action", "user", userOID.Hex(), "127.0.0.1", "test-agent")
}

func TestPostPublishService_NilServicesFaultTolerance(t *testing.T) {
	// Initialize with nil dependencies
	svc := service.NewPostPublishService(nil, nil, nil, nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic in PublishToPlatforms: %v", r)
		}
	}()

	userOID := bson.NewObjectID()
	post := &models.Post{ID: bson.NewObjectID()}

	// Empty platforms should return error safely
	_, err := svc.PublishToPlatforms(context.Background(), userOID, post, nil, nil)
	if err == nil {
		t.Error("expected error for empty selected platforms")
	}
}

func TestSocialService_NilConfigFaultTolerance(t *testing.T) {
	svc := service.NewSocialService(nil, nil, nil, nil)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic in GetConnectURL: %v", r)
		}
	}()

	// Should safely return error without panicking
	_, err := svc.GetConnectURL("user-123", "INSTAGRAM")
	if err == nil {
		t.Error("expected error for nil instagramService")
	}
}
