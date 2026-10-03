package migrations

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Migration defines the contract for versioned schema and index migrations.
type Migration interface {
	Version() int
	Name() string
	Up(ctx context.Context, db *mongo.Database) error
	Down(ctx context.Context, db *mongo.Database) error
}

// MigrationRecord represents an audit entry in the _schema_migrations collection.
type MigrationRecord struct {
	Version         int       `bson:"version" json:"version"`
	Name            string    `bson:"name" json:"name"`
	AppliedAt       time.Time `bson:"appliedAt" json:"appliedAt"`
	ExecutionTimeMs int64     `bson:"executionTimeMs" json:"executionTimeMs"`
	Checksum        string    `bson:"checksum" json:"checksum"`
	Status          string    `bson:"status" json:"status"` // "applied" or "rolled_back"
}

// MigrationStatus reports whether a registered migration has been applied.
type MigrationStatus struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	Applied   bool      `json:"applied"`
	AppliedAt time.Time `json:"appliedAt,omitempty"`
}
