package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/config"
)

type MongoDB struct {
	Client   *mongo.Client
	Database *mongo.Database
}

func ConnectMongoDB(cfg *config.Config) (*MongoDB, error) {
	if cfg.MongoDBName == "" {
		return nil, fmt.Errorf("MONGODB_DB_NAME is missing in environment variables")
	}
	if cfg.MongoURI == "" {
		return nil, fmt.Errorf("MONGODB_URI is missing in environment variables")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	clientOpts := options.Client().
		ApplyURI(cfg.MongoURI).
		SetMaxPoolSize(100).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(5 * time.Minute).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create MongoDB client: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}
	slog.Info("mongodb_connected_successfully")
	return &MongoDB{
		Client:   client,
		Database: client.Database(cfg.MongoDBName),
	}, nil
}
