package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"xsocial-backend/internal/models"
)

type WebhookRepository struct {
	collection *mongo.Collection
}

func NewWebhookRepository(db *mongo.Database) *WebhookRepository {
	repo := &WebhookRepository{
		collection: db.Collection("webhooks"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}}},
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
	})

	return repo
}

func (r *WebhookRepository) Create(ctx context.Context, wh *models.Webhook) error {
	now := time.Now()
	wh.CreatedAt = now
	wh.UpdatedAt = now
	if wh.Status == "" {
		wh.Status = "active"
	}
	res, err := r.collection.InsertOne(ctx, wh)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		wh.ID = oid
	}
	return nil
}

func (r *WebhookRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Webhook, error) {
	var wh models.Webhook
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&wh)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &wh, nil
}

func (r *WebhookRepository) FindByUserOrBrand(ctx context.Context, userID bson.ObjectID, brandID *bson.ObjectID) ([]models.Webhook, error) {
	filter := bson.M{"userId": userID}
	if brandID != nil {
		filter = bson.M{
			"$or": []bson.M{
				{"userId": userID},
				{"brandId": *brandID},
			},
		}
	}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var list []models.Webhook
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *WebhookRepository) Delete(ctx context.Context, id, userID bson.ObjectID) (bool, error) {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": id, "userId": userID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}
