package database

import (
	"context"
	"testing"

	"xsocial-backend/internal/config"
)

func TestConnectPostgres_EmptyConfig(t *testing.T) {
	cfg := &config.Config{PostgresURL: ""}
	db, err := ConnectPostgres(cfg)
	if err != nil {
		t.Fatalf("expected nil error for empty postgres URL, got %v", err)
	}
	if db != nil {
		t.Fatalf("expected nil PostgresDB when PostgresURL is empty, got %v", db)
	}
}

func TestPostgresDB_NilSafety(t *testing.T) {
	var db *PostgresDB
	ctx := context.Background()

	if err := db.Ping(ctx); err == nil {
		t.Errorf("expected error pinging nil db, got nil")
	}

	if err := db.Close(); err != nil {
		t.Errorf("expected nil error closing nil db, got %v", err)
	}

	if err := db.WithTransaction(ctx, nil); err == nil {
		t.Errorf("expected error running transaction on nil db, got nil")
	}

	if err := db.ExecuteSchema(ctx, ""); err == nil {
		t.Errorf("expected error executing schema on nil db, got nil")
	}
}
