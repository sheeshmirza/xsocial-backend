package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/models"
)

type RefreshTokenRepository struct {
	collection *mongo.Collection
}

func NewRefreshTokenRepository(db *mongo.Database) *RefreshTokenRepository {
	repo := &RefreshTokenRepository{
		collection: db.Collection("refreshtokens"),
	}
	_ = repo.ensureIndexes(context.Background())
	return repo
}

func (r *RefreshTokenRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tokenHash", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "tokenExpiresAt", Value: 1}},
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	now := time.Now()
	token.CreatedAt = now
	token.UpdatedAt = now
	res, err := r.collection.InsertOne(ctx, token)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		token.ID = oid
	}
	return nil
}

func (r *RefreshTokenRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	var token models.RefreshToken
	err := r.collection.FindOne(ctx, bson.M{"tokenHash": tokenHash}).Decode(&token)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *RefreshTokenRepository) RevokeByID(ctx context.Context, id bson.ObjectID) error {
	now := time.Now()
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"tokenRevokedAt": now,
			"updatedAt":      now,
		},
	})
	return err
}

func (r *RefreshTokenRepository) RevokeAllByUserID(ctx context.Context, userID bson.ObjectID) error {
	now := time.Now()
	_, err := r.collection.UpdateMany(ctx, bson.M{
		"userId":         userID,
		"tokenRevokedAt": nil,
	}, bson.M{
		"$set": bson.M{
			"tokenRevokedAt": now,
			"updatedAt":      now,
		},
	})
	return err
}

func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{
		"tokenExpiresAt": bson.M{"$lt": now},
	})
	return err
}
