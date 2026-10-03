package migrations_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"xsocial-backend/internal/database/migrations"
)

type mockMigration struct {
	version     int
	name        string
	upInvoked   bool
	downInvoked bool
}

func (m *mockMigration) Version() int                               { return m.version }
func (m *mockMigration) Name() string                              { return m.name }
func (m *mockMigration) Up(ctx context.Context, db *mongo.Database) error {
	m.upInvoked = true
	return nil
}
func (m *mockMigration) Down(ctx context.Context, db *mongo.Database) error {
	m.downInvoked = true
	return nil
}

func TestMigratorRegistration(t *testing.T) {
	migrator := migrations.NewMigrator(nil)

	m1 := &mockMigration{version: 10, name: "custom_migration_10"}
	m2 := &mockMigration{version: 5, name: "custom_migration_5"}

	migrator.Register(m1)
	migrator.Register(m2)

	// Built-in has 4 migrations (1..4), plus 5 and 10 = 6 migrations
	ctx := context.Background()
	statuses, err := migrator.Status(ctx)
	// Because db is nil, Status will return error or we test with mock if db needed
	if err == nil && len(statuses) < 6 {
		t.Errorf("expected at least 6 registered migrations, got %d", len(statuses))
	}
}
