package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/models"
)

type AnalyticsRepository struct {
	webEventsCollection     *mongo.Collection
	socialRollupsCollection *mongo.Collection
}

func NewAnalyticsRepository(db *mongo.Database) *AnalyticsRepository {
	repo := &AnalyticsRepository{
		webEventsCollection:     db.Collection("web_analytics_events"),
		socialRollupsCollection: db.Collection("social_metric_rollups"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.webEventsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "timestamp", Value: -1}}},
	})
	_, _ = repo.socialRollupsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "brandId", Value: 1}, {Key: "platform", Value: 1}, {Key: "date", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "date", Value: -1}}},
	})

	return repo
}

func (r *AnalyticsRepository) RecordWebEvent(ctx context.Context, event *models.WebEvent) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	res, err := r.webEventsCollection.InsertOne(ctx, event)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		event.ID = oid
	}
	return nil
}

func (r *AnalyticsRepository) GetWebMetrics(ctx context.Context, brandID bson.ObjectID, from, to time.Time) (map[string]any, error) {
	filter := bson.M{
		"brandId": brandID,
		"timestamp": bson.M{
			"$gte": from,
			"$lte": to,
		},
	}
	totalPageviews, err := r.webEventsCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	var distinctSessions []string
	_ = r.webEventsCollection.Distinct(ctx, "sessionId", filter).Decode(&distinctSessions)
	visitorsCount := int64(len(distinctSessions))

	return map[string]any{
		"pageviews": totalPageviews,
		"visitors":  visitorsCount,
		"from":      from,
		"to":        to,
	}, nil
}

func (r *AnalyticsRepository) UpsertSocialRollup(ctx context.Context, rollup *models.SocialMetricRollup) error {
	rollup.CreatedAt = time.Now()
	filter := bson.M{
		"brandId":  rollup.BrandID,
		"platform": rollup.Platform,
		"date":     rollup.Date,
	}
	update := bson.M{
		"$set": bson.M{
			"followers":         rollup.Followers,
			"followersGained":   rollup.FollowersGained,
			"impressions":       rollup.Impressions,
			"reach":             rollup.Reach,
			"engagement":        rollup.Engagement,
			"clicks":            rollup.Clicks,
			"videoViews":        rollup.VideoViews,
			"postsCount":        rollup.PostsCount,
			"interactionsCount": rollup.InteractionsCount,
			"createdAt":         rollup.CreatedAt,
		},
	}
	opt := options.UpdateOne().SetUpsert(true)
	_, err := r.socialRollupsCollection.UpdateOne(ctx, filter, update, opt)
	return err
}

func (r *AnalyticsRepository) GetSocialMetrics(ctx context.Context, brandID bson.ObjectID, platform string, fromDate, toDate string) ([]models.SocialMetricRollup, error) {
	filter := bson.M{"brandId": brandID}
	if platform != "" {
		filter["platform"] = platform
	}
	if fromDate != "" && toDate != "" {
		filter["date"] = bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		}
	}
	cursor, err := r.socialRollupsCollection.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "date", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rollups []models.SocialMetricRollup
	if err := cursor.All(ctx, &rollups); err != nil {
		return nil, err
	}
	return rollups, nil
}
