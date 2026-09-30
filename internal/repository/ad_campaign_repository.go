package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"xsocial-backend/internal/models"
)

type AdCampaignRepository struct {
	collection *mongo.Collection
}

func NewAdCampaignRepository(db *mongo.Database) *AdCampaignRepository {
	repo := &AdCampaignRepository{
		collection: db.Collection("ad_campaigns"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "brandId", Value: 1}, {Key: "externalCampaignId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "platform", Value: 1}}},
	})

	return repo
}

func (r *AdCampaignRepository) UpsertCampaign(ctx context.Context, camp *models.AdCampaign) error {
	now := time.Now()
	camp.UpdatedAt = now
	filter := bson.M{"brandId": camp.BrandID, "externalCampaignId": camp.ExternalCampaignID}
	update := bson.M{
		"$set": bson.M{
			"name":        camp.Name,
			"status":      camp.Status,
			"objective":   camp.Objective,
			"budget":      camp.Budget,
			"spend":       camp.Spend,
			"impressions": camp.Impressions,
			"clicks":      camp.Clicks,
			"conversions": camp.Conversions,
			"ctr":         camp.CTR,
			"cpc":         camp.CPC,
			"cpm":         camp.CPM,
			"roas":        camp.ROAS,
			"dateRange":   camp.DateRange,
			"updatedAt":   now,
		},
		"$setOnInsert": bson.M{
			"platform":  camp.Platform,
			"createdAt": now,
		},
	}
	opt := options.UpdateOne().SetUpsert(true)
	_, err := r.collection.UpdateOne(ctx, filter, update, opt)
	return err
}

func (r *AdCampaignRepository) FindByBrand(ctx context.Context, brandID bson.ObjectID, platform string) ([]models.AdCampaign, error) {
	filter := bson.M{"brandId": brandID}
	if platform != "" {
		filter["platform"] = platform
	}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var list []models.AdCampaign
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *AdCampaignRepository) GetSummary(ctx context.Context, brandID bson.ObjectID) (*models.AdSummary, error) {
	camps, err := r.FindByBrand(ctx, brandID, "")
	if err != nil {
		return nil, err
	}

	summary := &models.AdSummary{}
	if len(camps) == 0 {
		return summary, nil
	}

	for _, c := range camps {
		summary.TotalSpend += c.Spend
		summary.TotalImpressions += c.Impressions
		summary.TotalClicks += c.Clicks
		summary.TotalConversions += c.Conversions
	}

	if summary.TotalImpressions > 0 {
		summary.AverageCTR = (float64(summary.TotalClicks) / float64(summary.TotalImpressions)) * 100
	}
	if summary.TotalClicks > 0 {
		summary.AverageCPC = summary.TotalSpend / float64(summary.TotalClicks)
	}
	if summary.TotalSpend > 0 && summary.TotalConversions > 0 {
		summary.AverageROAS = float64(summary.TotalConversions*50) / summary.TotalSpend // approximate conversion value calculation
	}
	return summary, nil
}

func (r *AdCampaignRepository) Create(ctx context.Context, camp *models.AdCampaign) error {
	now := time.Now()
	camp.CreatedAt = now
	camp.UpdatedAt = now
	res, err := r.collection.InsertOne(ctx, camp)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		camp.ID = oid
	}
	return nil
}

func (r *AdCampaignRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.AdCampaign, error) {
	var camp models.AdCampaign
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&camp)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &camp, nil
}

func (r *AdCampaignRepository) Update(ctx context.Context, id, brandID bson.ObjectID, update bson.M) (*models.AdCampaign, error) {
	update["updatedAt"] = time.Now()
	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.AdCampaign
	err := r.collection.FindOneAndUpdate(ctx, bson.M{"_id": id, "brandId": brandID}, bson.M{"$set": update}, opt).Decode(&updated)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &updated, nil
}

func (r *AdCampaignRepository) Delete(ctx context.Context, id, brandID bson.ObjectID) (bool, error) {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": id, "brandId": brandID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

