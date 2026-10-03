package repository

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/models"
)

type SocialHandleRepository struct {
	collection *mongo.Collection
}

func NewSocialHandleRepository(db *mongo.Database) *SocialHandleRepository {
	repo := &SocialHandleRepository{
		collection: db.Collection("socialhandles"),
	}
	_ = repo.EnsureIndexes(context.Background())
	return repo
}

func (r *SocialHandleRepository) EnsureIndexes(ctx context.Context) error {
	// Drop deprecated index if present
	_ = r.collection.Indexes().DropOne(ctx, "uniq_user_platform")

	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "platform", Value: 1},
				{Key: "platformId", Value: 1},
			},
			Options: options.Index().SetUnique(true).SetName("uniq_user_platform_identity"),
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "brandId", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "platformAccessTokenExpiry", Value: 1}},
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexModels)
	return err
}

func (r *SocialHandleRepository) Create(ctx context.Context, handle *models.SocialHandle) error {
	now := time.Now()
	handle.CreatedAt = now
	handle.UpdatedAt = now
	handle.Platform = strings.ToUpper(strings.TrimSpace(handle.Platform))

	res, err := r.collection.InsertOne(ctx, handle)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		handle.ID = oid
	}
	return nil
}

func (r *SocialHandleRepository) ListByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M, skip, limit int, sort bson.D) ([]models.SocialHandle, error) {
	query := bson.M{"userId": userID}
	for k, v := range filters {
		query[k] = v
	}

	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit))
	if len(sort) > 0 {
		opts.SetSort(sort)
	} else {
		opts.SetSort(bson.D{{Key: "createdAt", Value: -1}})
	}

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.SocialHandle
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.SocialHandle{}
	}
	return items, nil
}

func (r *SocialHandleRepository) CountByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M) (int64, error) {
	query := bson.M{"userId": userID}
	for k, v := range filters {
		query[k] = v
	}
	return r.collection.CountDocuments(ctx, query)
}

func (r *SocialHandleRepository) FindByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.SocialHandle, error) {
	var handle models.SocialHandle
	err := r.collection.FindOne(ctx, bson.M{"_id": id, "userId": userID}).Decode(&handle)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &handle, nil
}

func (r *SocialHandleRepository) UpsertByUserPlatformIdentity(ctx context.Context, userID bson.ObjectID, platform, platformID string, handle *models.SocialHandle) (*models.SocialHandle, error) {
	_ = r.EnsureIndexes(ctx)

	normPlatform := strings.ToUpper(strings.TrimSpace(platform))
	filter := bson.M{
		"userId":     userID,
		"platform":   normPlatform,
		"platformId": strings.TrimSpace(platformID),
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"platformUsername":           handle.PlatformUsername,
			"platformDisplayName":        handle.PlatformDisplayName,
			"platformAccessToken":        handle.PlatformAccessToken,
			"platformRefreshToken":       handle.PlatformRefreshToken,
			"platformAccessTokenExpiry":  handle.PlatformAccessTokenExpiry,
			"platformRefreshTokenExpiry": handle.PlatformRefreshTokenExpiry,
			"platformMetadata":           handle.PlatformMetadata,
			"updatedAt":                  now,
		},
		"$setOnInsert": bson.M{
			"userId":     userID,
			"platform":   normPlatform,
			"platformId": strings.TrimSpace(platformID),
			"createdAt":  now,
		},
	}

	opt := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var updated models.SocialHandle
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opt).Decode(&updated)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *SocialHandleRepository) UpdateByIDAndUserID(ctx context.Context, id, userID bson.ObjectID, update bson.M) (*models.SocialHandle, error) {
	update["updatedAt"] = time.Now()
	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.SocialHandle
	err := r.collection.FindOneAndUpdate(ctx, bson.M{"_id": id, "userId": userID}, bson.M{"$set": update}, opt).Decode(&updated)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &updated, nil
}

func (r *SocialHandleRepository) DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.SocialHandle, error) {
	var deleted models.SocialHandle
	err := r.collection.FindOneAndDelete(ctx, bson.M{"_id": id, "userId": userID}).Decode(&deleted)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &deleted, nil
}

func (r *SocialHandleRepository) FindByUserPlatform(ctx context.Context, userID bson.ObjectID, platform string) ([]models.SocialHandle, error) {
	cursor, err := r.collection.Find(ctx, bson.M{
		"userId":   userID,
		"platform": strings.ToUpper(strings.TrimSpace(platform)),
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.SocialHandle
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.SocialHandle{}
	}
	return items, nil
}

func (r *SocialHandleRepository) FindByUserPlatforms(ctx context.Context, userID bson.ObjectID, platformList []string) ([]models.SocialHandle, error) {
	normalized := make([]string, 0, len(platformList))
	for _, p := range platformList {
		normalized = append(normalized, strings.ToUpper(strings.TrimSpace(p)))
	}

	cursor, err := r.collection.Find(ctx, bson.M{
		"userId":   userID,
		"platform": bson.M{"$in": normalized},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.SocialHandle
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.SocialHandle{}
	}
	return items, nil
}

func (r *SocialHandleRepository) FindByUserAndIDs(ctx context.Context, userID bson.ObjectID, ids []string) ([]models.SocialHandle, error) {
	objectIDs := make([]bson.ObjectID, 0, len(ids))
	for _, idStr := range ids {
		if oid, err := bson.ObjectIDFromHex(idStr); err == nil {
			objectIDs = append(objectIDs, oid)
		}
	}

	if len(objectIDs) == 0 {
		return []models.SocialHandle{}, nil
	}

	cursor, err := r.collection.Find(ctx, bson.M{
		"userId": userID,
		"_id":    bson.M{"$in": objectIDs},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.SocialHandle
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.SocialHandle{}
	}
	return items, nil
}

func (r *SocialHandleRepository) FindByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.SocialHandle, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"brandId": brandID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.SocialHandle
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.SocialHandle{}
	}
	return items, nil
}

