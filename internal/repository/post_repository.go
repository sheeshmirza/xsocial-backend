package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/models"
)

type StatusBreakdownResult struct {
	Status string `bson:"_id" json:"status"`
	Count  int64  `bson:"count" json:"count"`
}

type PlatformStatsResult struct {
	Platform string `bson:"_id" json:"platform"`
	Count    int64  `bson:"count" json:"count"`
}

type PostRepository struct {
	collection *mongo.Collection
}

func NewPostRepository(db *mongo.Database) *PostRepository {
	repo := &PostRepository{
		collection: db.Collection("posts"),
	}
	_ = repo.ensureIndexes(context.Background())
	return repo
}

func (r *PostRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "createdAt", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "scheduledTime", Value: 1}, {Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "publishedExpiresAt", Value: 1}},
		},
	}
	_, err := r.collection.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *PostRepository) Create(ctx context.Context, post *models.Post) error {
	now := time.Now()
	post.CreatedAt = now
	post.UpdatedAt = now

	res, err := r.collection.InsertOne(ctx, post)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		post.ID = oid
	}
	return nil
}

func (r *PostRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Post, error) {
	var post models.Post
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (r *PostRepository) FindByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.Post, error) {
	var post models.Post
	err := r.collection.FindOne(ctx, bson.M{"_id": id, "userId": userID}).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (r *PostRepository) ListRecurringScheduledPosts(ctx context.Context) ([]models.Post, error) {
	query := bson.M{
		"status":           models.PostStatusScheduled,
		"recurrence":       bson.M{"$ne": "once"},
		"recurrencePaused": bson.M{"$ne": true},
		"scheduledTime":    bson.M{"$ne": nil},
	}
	opts := options.Find().SetProjection(bson.M{"_id": 1, "scheduledTime": 1, "userId": 1, "recurrence": 1, "recurrenceInterval": 1, "recurrenceUnit": 1})
	cursor, err := r.collection.Find(ctx, query, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var posts []models.Post
	if err := cursor.All(ctx, &posts); err != nil {
		return nil, err
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

func (r *PostRepository) ListByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M, skip, limit int, sort bson.D) ([]models.Post, error) {
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

	var items []models.Post
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.Post{}
	}
	return items, nil
}

func (r *PostRepository) CountByUserID(ctx context.Context, userID bson.ObjectID, filters bson.M) (int64, error) {
	query := bson.M{"userId": userID}
	for k, v := range filters {
		query[k] = v
	}
	return r.collection.CountDocuments(ctx, query)
}

func (r *PostRepository) UpdateByIDAndUserID(ctx context.Context, id, userID bson.ObjectID, update bson.M) (*models.Post, error) {
	update["updatedAt"] = time.Now()
	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var post models.Post
	err := r.collection.FindOneAndUpdate(ctx, bson.M{"_id": id, "userId": userID}, bson.M{"$set": update}, opt).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (r *PostRepository) DeleteByIDAndUserID(ctx context.Context, id, userID bson.ObjectID) (*models.Post, error) {
	var post models.Post
	err := r.collection.FindOneAndDelete(ctx, bson.M{"_id": id, "userId": userID}).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (r *PostRepository) DeleteManyByIDsAndUserID(ctx context.Context, ids []string, userID bson.ObjectID) (int64, error) {
	objectIDs := make([]bson.ObjectID, 0, len(ids))
	for _, idStr := range ids {
		if oid, err := bson.ObjectIDFromHex(idStr); err == nil {
			objectIDs = append(objectIDs, oid)
		}
	}
	if len(objectIDs) == 0 {
		return 0, nil
	}

	res, err := r.collection.DeleteMany(ctx, bson.M{
		"_id":    bson.M{"$in": objectIDs},
		"userId": userID,
	})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (r *PostRepository) FindByIDsAndUserID(ctx context.Context, ids []string, userID bson.ObjectID) ([]models.Post, error) {
	objectIDs := make([]bson.ObjectID, 0, len(ids))
	for _, idStr := range ids {
		if oid, err := bson.ObjectIDFromHex(idStr); err == nil {
			objectIDs = append(objectIDs, oid)
		}
	}
	if len(objectIDs) == 0 {
		return []models.Post{}, nil
	}

	cursor, err := r.collection.Find(ctx, bson.M{
		"_id":    bson.M{"$in": objectIDs},
		"userId": userID,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var items []models.Post
	if err := cursor.All(ctx, &items); err != nil {
		return nil, err
	}
	if items == nil {
		items = []models.Post{}
	}
	return items, nil
}

func (r *PostRepository) AggregateDashboard(ctx context.Context, userID bson.ObjectID, filters bson.M) (map[string]int64, error) {
	match := bson.M{"userId": userID}
	for k, v := range filters {
		match[k] = v
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []StatusBreakdownResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	breakdown := map[string]int64{
		"draft":     0,
		"scheduled": 0,
		"published": 0,
		"failed":    0,
	}
	for _, row := range results {
		breakdown[row.Status] = row.Count
	}
	return breakdown, nil
}

func (r *PostRepository) PlatformStats(ctx context.Context, userID bson.ObjectID, filters bson.M) ([]PlatformStatsResult, error) {
	match := bson.M{"userId": userID}
	for k, v := range filters {
		match[k] = v
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$unwind", Value: "$selectedPlatforms"}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$selectedPlatforms",
			"count": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []PlatformStatsResult
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	if results == nil {
		results = []PlatformStatsResult{}
	}
	return results, nil
}

func (r *PostRepository) FindByApprovalToken(ctx context.Context, token string) (*models.Post, error) {
	var post models.Post
	err := r.collection.FindOne(ctx, bson.M{"approvalToken": token}).Decode(&post)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}
