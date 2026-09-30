package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/models"
)

type AuditLogRepository struct {
	collection *mongo.Collection
}

func NewAuditLogRepository(db *mongo.Database) *AuditLogRepository {
	repo := &AuditLogRepository{
		collection: db.Collection("audit_logs"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}}},
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "createdAt", Value: -1}}},
	})

	return repo
}

func (r *AuditLogRepository) Record(ctx context.Context, log *models.AuditLog) {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	_, _ = r.collection.InsertOne(ctx, log)
}

func (r *AuditLogRepository) Find(ctx context.Context, userID *bson.ObjectID, brandID *bson.ObjectID, limit int64) ([]models.AuditLog, error) {
	filter := bson.M{}
	if userID != nil {
		filter["userId"] = *userID
	}
	if brandID != nil {
		filter["brandId"] = *brandID
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(limit)
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var list []models.AuditLog
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	return list, nil
}
