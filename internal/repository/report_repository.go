package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/models"
)

type ReportRepository struct {
	collection *mongo.Collection
}

func NewReportRepository(db *mongo.Database) *ReportRepository {
	repo := &ReportRepository{
		collection: db.Collection("reports"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "createdAt", Value: -1}}},
	})

	return repo
}

func (r *ReportRepository) Create(ctx context.Context, report *models.Report) error {
	now := time.Now()
	report.CreatedAt = now
	report.Status = models.ReportStatusPending
	res, err := r.collection.InsertOne(ctx, report)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		report.ID = oid
	}
	return nil
}

func (r *ReportRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Report, error) {
	var rep models.Report
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&rep)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &rep, nil
}

func (r *ReportRepository) FindByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.Report, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"brandId": brandID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var list []models.Report
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *ReportRepository) UpdateStatus(ctx context.Context, id bson.ObjectID, status models.ReportStatus, fileURL string) error {
	now := time.Now()
	update := bson.M{
		"status":      status,
		"fileUrl":     fileURL,
		"completedAt": now,
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *ReportRepository) UpdateWithMetrics(ctx context.Context, id bson.ObjectID, status models.ReportStatus, fileURL string, metrics map[string]any) error {
	now := time.Now()
	update := bson.M{
		"status":      status,
		"fileUrl":     fileURL,
		"metrics":     metrics,
		"completedAt": now,
	}
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

