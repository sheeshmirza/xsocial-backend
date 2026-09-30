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

type PostActivityRepository struct {
	collection *mongo.Collection
}

func NewPostActivityRepository(db *mongo.Database) *PostActivityRepository {
	repo := &PostActivityRepository{
		collection: db.Collection("postactivities"),
	}
	_ = repo.ensureIndexes(context.Background())
	return repo
}

func (r *PostActivityRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "socialHandleId", Value: 1},
				{Key: "platform", Value: 1},
				{Key: "platformPostId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "updatedAt", Value: -1}},
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *PostActivityRepository) UpsertLinkedInActivity(
	ctx context.Context,
	userID, socialHandleID bson.ObjectID,
	platformPostID string,
	activity *models.PostActivity,
) (*models.PostActivity, error) {
	normPostID := strings.TrimSpace(platformPostID)
	filter := bson.M{
		"userId":         userID,
		"socialHandleId": socialHandleID,
		"platform":       "linkedin",
		"platformPostId": normPostID,
	}

	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"platformPostUrn": activity.PlatformPostURN,
			"platformPostUrl": activity.PlatformPostURL,
			"contentSnapshot": activity.ContentSnapshot,
			"updatedAt":       now,
		},
		"$setOnInsert": bson.M{
			"userId":         userID,
			"socialHandleId": socialHandleID,
			"platform":       "linkedin",
			"platformPostId": normPostID,
			"createdAt":      now,
		},
	}

	opt := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var updated models.PostActivity
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opt).Decode(&updated)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *PostActivityRepository) ListByPlatformPostIDs(
	ctx context.Context,
	userID bson.ObjectID,
	platformPostIDs []string,
	platform string,
) ([]models.PostActivity, error) {
	if len(platformPostIDs) == 0 {
		return []models.PostActivity{}, nil
	}

	cursor, err := r.collection.Find(ctx, bson.M{
		"userId":         userID,
		"platform":       strings.ToLower(strings.TrimSpace(platform)),
		"platformPostId": bson.M{"$in": platformPostIDs},
	}, options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.PostActivity
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.PostActivity{}
	}
	return items, nil
}

func (r *PostActivityRepository) DeleteByPlatformPostIDs(
	ctx context.Context,
	userID bson.ObjectID,
	platformPostIDs []string,
	platform string,
) error {
	if len(platformPostIDs) == 0 {
		return nil
	}

	_, err := r.collection.DeleteMany(ctx, bson.M{
		"userId":         userID,
		"platform":       strings.ToLower(strings.TrimSpace(platform)),
		"platformPostId": bson.M{"$in": platformPostIDs},
	})
	return err
}

func (r *PostActivityRepository) DeleteByPostPlatformAndPlatformPostID(
	ctx context.Context,
	userID bson.ObjectID,
	platform, platformPostID string,
) error {
	_, err := r.collection.DeleteMany(ctx, bson.M{
		"userId":         userID,
		"platform":       strings.ToLower(strings.TrimSpace(platform)),
		"platformPostId": strings.TrimSpace(platformPostID),
	})
	return err
}

func (r *PostActivityRepository) ListByUser(
	ctx context.Context,
	userID bson.ObjectID,
	filters bson.M,
	skip, limit int,
	sort bson.D,
) ([]models.PostActivity, error) {
	query := bson.M{"userId": userID}
	for k, v := range filters {
		query[k] = v
	}

	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(limit))
	if len(sort) > 0 {
		opts.SetSort(sort)
	} else {
		opts.SetSort(bson.D{{Key: "updatedAt", Value: -1}})
	}

	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.PostActivity
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.PostActivity{}
	}
	return items, nil
}

func (r *PostActivityRepository) CountByUser(ctx context.Context, userID bson.ObjectID, filters bson.M) (int64, error) {
	query := bson.M{"userId": userID}
	for k, v := range filters {
		query[k] = v
	}
	return r.collection.CountDocuments(ctx, query)
}
