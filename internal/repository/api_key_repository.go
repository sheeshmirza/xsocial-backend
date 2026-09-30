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

type ApiKeyRepository struct {
	collection *mongo.Collection
}

func NewApiKeyRepository(db *mongo.Database) *ApiKeyRepository {
	repo := &ApiKeyRepository{
		collection: db.Collection("api_keys"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "keyHash", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "brandId", Value: 1}},
		},
	})

	return repo
}

func (r *ApiKeyRepository) Create(ctx context.Context, key *models.ApiKey) error {
	now := time.Now()
	key.CreatedAt = now
	key.UpdatedAt = now
	res, err := r.collection.InsertOne(ctx, key)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		key.ID = oid
	}
	return nil
}

func (r *ApiKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*models.ApiKey, error) {
	var key models.ApiKey
	err := r.collection.FindOne(ctx, bson.M{"keyHash": keyHash}).Decode(&key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}

func (r *ApiKeyRepository) FindByUserID(ctx context.Context, userID bson.ObjectID) ([]models.ApiKey, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"userId": userID}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var keys []models.ApiKey
	if err := cursor.All(ctx, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func (r *ApiKeyRepository) DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (bool, error) {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": id, "userId": userID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

func (r *ApiKeyRepository) UpdateLastUsed(ctx context.Context, id bson.ObjectID) {
	now := time.Now()
	_, _ = r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"lastUsedAt": now, "updatedAt": now}})
}
