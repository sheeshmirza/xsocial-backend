package database

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"

	"xsocial-backend/internal/config"
)

//go:embed migrations/sql/001_normalized_relational_schema.sql
var DefaultPostgresSchemaSQL string

// PostgresDB encapsulates the relational database connection pool and query execution methods.
type PostgresDB struct {
	DB *sql.DB
}

// ConnectPostgres initializes a connection pool to PostgreSQL using production connection parameters.
// If PostgresURL is empty, it returns (nil, nil) allowing optional relational engine enablement.
func ConnectPostgres(cfg *config.Config) (*PostgresDB, error) {
	if cfg.PostgresURL == "" {
		return nil, nil
	}

	db, err := sql.Open("postgres", cfg.PostgresURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	// High-throughput connection pool configuration
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(1 * time.Hour)
	db.SetConnMaxIdleTime(15 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	slog.Info("postgresql_connected_successfully", "max_open_conns", 100, "max_idle_conns", 25)
	return &PostgresDB{DB: db}, nil
}

// Ping verifies connectivity to the PostgreSQL cluster with a timeout context.
func (p *PostgresDB) Ping(ctx context.Context) error {
	if p == nil || p.DB == nil {
		return fmt.Errorf("postgresql client is uninitialized")
	}
	return p.DB.PingContext(ctx)
}

// Close gracefully terminates the connection pool.
func (p *PostgresDB) Close() error {
	if p == nil || p.DB == nil {
		return nil
	}
	return p.DB.Close()
}

// WithTransaction runs the given function inside a PostgreSQL ACID transaction.
// Automatically rolls back if an error occurs or on panic, and commits on success.
func (p *PostgresDB) WithTransaction(ctx context.Context, fn func(tx *sql.Tx) error) (err error) {
	if p == nil || p.DB == nil {
		return fmt.Errorf("postgresql database is uninitialized")
	}

	tx, err := p.DB.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return fmt.Errorf("failed to begin postgres transaction: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		} else if err != nil {
			_ = tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// ExecuteSchema executes multi-statement SQL strings for migrations and schema bootstrapping.
func (p *PostgresDB) ExecuteSchema(ctx context.Context, schemaSQL string) error {
	if p == nil || p.DB == nil {
		return fmt.Errorf("postgresql database is uninitialized")
	}
	_, err := p.DB.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("failed to execute postgres schema: %w", err)
	}
	return nil
}

// AutoMigrate applies the built-in 3NF normalized schema migrations to the PostgreSQL cluster.
func (p *PostgresDB) AutoMigrate(ctx context.Context) error {
	if p == nil || p.DB == nil {
		return nil
	}
	if DefaultPostgresSchemaSQL == "" {
		return nil
	}
	slog.Info("applying_postgresql_relational_migrations")
	return p.ExecuteSchema(ctx, DefaultPostgresSchemaSQL)
}
