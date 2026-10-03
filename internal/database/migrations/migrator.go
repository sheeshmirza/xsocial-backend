package migrations

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const MigrationsCollection = "_schema_migrations"

type Migrator struct {
	db         *mongo.Database
	migrations []Migration
}

func NewMigrator(db *mongo.Database) *Migrator {
	m := &Migrator{
		db:         db,
		migrations: make([]Migration, 0),
	}
	m.registerBuiltInMigrations()
	return m
}

func (m *Migrator) Register(migration Migration) {
	m.migrations = append(m.migrations, migration)
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version() < m.migrations[j].Version()
	})
}

func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	coll := m.db.Collection(MigrationsCollection)
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := coll.Indexes().CreateOne(ctx, indexModel)
	return err
}

func (m *Migrator) getAppliedVersions(ctx context.Context) (map[int]MigrationRecord, error) {
	coll := m.db.Collection(MigrationsCollection)
	cursor, err := coll.Find(ctx, bson.M{"status": "applied"})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	applied := make(map[int]MigrationRecord)
	for cursor.Next(ctx) {
		var rec MigrationRecord
		if err := cursor.Decode(&rec); err == nil {
			applied[rec.Version] = rec
		}
	}
	return applied, nil
}

// ApplyAll applies all pending migrations in ascending version order.
func (m *Migrator) ApplyAll(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to prepare migrations collection: %w", err)
	}

	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("failed to query applied migrations: %w", err)
	}

	appliedCount := 0
	for _, mig := range m.migrations {
		if _, exists := applied[mig.Version()]; exists {
			continue
		}

		start := time.Now()
		log.Printf("[Migration] Running migration %03d: %s...", mig.Version(), mig.Name())

		if err := mig.Up(ctx, m.db); err != nil {
			return fmt.Errorf("migration %03d (%s) failed: %w", mig.Version(), mig.Name(), err)
		}

		duration := time.Since(start).Milliseconds()
		record := MigrationRecord{
			Version:         mig.Version(),
			Name:            mig.Name(),
			AppliedAt:       time.Now(),
			ExecutionTimeMs: duration,
			Status:          "applied",
		}

		coll := m.db.Collection(MigrationsCollection)
		_, err := coll.UpdateOne(
			ctx,
			bson.M{"version": mig.Version()},
			bson.M{"$set": record},
			options.UpdateOne().SetUpsert(true),
		)
		if err != nil {
			return fmt.Errorf("failed to record applied migration %03d: %w", mig.Version(), err)
		}

		log.Printf("[Migration] Applied migration %03d: %s (%dms)", mig.Version(), mig.Name(), duration)
		appliedCount++
	}

	if appliedCount == 0 {
		log.Println("[Migration] Database schema is up to date (no pending migrations)")
	} else {
		log.Printf("[Migration] Successfully applied %d migration(s)", appliedCount)
	}

	return nil
}

// RollbackLast rolls back the latest applied migration.
func (m *Migrator) RollbackLast(ctx context.Context) error {
	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		log.Println("[Migration] No applied migrations to rollback")
		return nil
	}

	// Find the latest applied version
	var latestVersion int
	for v := range applied {
		if v > latestVersion {
			latestVersion = v
		}
	}

	var targetMigration Migration
	for _, mig := range m.migrations {
		if mig.Version() == latestVersion {
			targetMigration = mig
			break
		}
	}

	if targetMigration == nil {
		return fmt.Errorf("migration version %d not found in registered migrations", latestVersion)
	}

	log.Printf("[Migration] Rolling back migration %03d: %s...", targetMigration.Version(), targetMigration.Name())
	if err := targetMigration.Down(ctx, m.db); err != nil {
		return fmt.Errorf("rollback failed for migration %03d: %w", targetMigration.Version(), err)
	}

	coll := m.db.Collection(MigrationsCollection)
	_, err = coll.UpdateOne(
		ctx,
		bson.M{"version": latestVersion},
		bson.M{"$set": bson.M{
			"status":       "rolled_back",
			"rolledBackAt": time.Now(),
		}},
	)
	if err != nil {
		return fmt.Errorf("failed to update migration record: %w", err)
	}

	log.Printf("[Migration] Rolled back migration %03d: %s", targetMigration.Version(), targetMigration.Name())
	return nil
}

// Status returns the current status of all registered migrations.
func (m *Migrator) Status(ctx context.Context) ([]MigrationStatus, error) {
	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return nil, err
	}

	statuses := make([]MigrationStatus, 0, len(m.migrations))
	for _, mig := range m.migrations {
		rec, isApplied := applied[mig.Version()]
		s := MigrationStatus{
			Version: mig.Version(),
			Name:    mig.Name(),
			Applied: isApplied,
		}
		if isApplied {
			s.AppliedAt = rec.AppliedAt
		}
		statuses = append(statuses, s)
	}
	return statuses, nil
}
