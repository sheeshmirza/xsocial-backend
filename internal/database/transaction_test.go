package database_test

import (
	"context"
	"testing"

	"xsocial-backend/internal/database"
)

func TestWithTransaction_NilClient(t *testing.T) {
	ctx := context.Background()
	invoked := false

	res, err := database.WithTransaction(ctx, nil, func(sessCtx context.Context) (any, error) {
		invoked = true
		return "success", nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !invoked {
		t.Error("expected transaction callback to be invoked")
	}
	if res != "success" {
		t.Errorf("expected result 'success', got %v", res)
	}
}
