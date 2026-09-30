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

type SmartLinkRepository struct {
	collection *mongo.Collection
}

func NewSmartLinkRepository(db *mongo.Database) *SmartLinkRepository {
	repo := &SmartLinkRepository{
		collection: db.Collection("smart_links"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
	})

	return repo
}

func (r *SmartLinkRepository) CreatePage(ctx context.Context, page *models.SmartLinkPage) error {
	now := time.Now()
	page.CreatedAt = now
	page.UpdatedAt = now
	page.IsActive = true
	res, err := r.collection.InsertOne(ctx, page)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		page.ID = oid
	}
	return nil
}

func (r *SmartLinkRepository) FindPageByID(ctx context.Context, id bson.ObjectID) (*models.SmartLinkPage, error) {
	var page models.SmartLinkPage
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&page)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &page, nil
}

func (r *SmartLinkRepository) FindPageBySlug(ctx context.Context, slug string) (*models.SmartLinkPage, error) {
	var page models.SmartLinkPage
	err := r.collection.FindOne(ctx, bson.M{"slug": slug, "isActive": true}).Decode(&page)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &page, nil
}

func (r *SmartLinkRepository) FindPagesByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.SmartLinkPage, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"brandId": brandID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var pages []models.SmartLinkPage
	if err := cursor.All(ctx, &pages); err != nil {
		return nil, err
	}
	return pages, nil
}

func (r *SmartLinkRepository) UpdatePage(ctx context.Context, id bson.ObjectID, update bson.M) (*models.SmartLinkPage, error) {
	update["updatedAt"] = time.Now()
	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.SmartLinkPage
	err := r.collection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": update}, opt).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &updated, nil
}

func (r *SmartLinkRepository) DeletePage(ctx context.Context, id, brandID bson.ObjectID) (bool, error) {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": id, "brandId": brandID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

func (r *SmartLinkRepository) IncrementViews(ctx context.Context, slug string) {
	_, _ = r.collection.UpdateOne(ctx, bson.M{"slug": slug}, bson.M{"$inc": bson.M{"totalViews": 1}})
}

func (r *SmartLinkRepository) IncrementClicks(ctx context.Context, slug, buttonID string) {
	_, _ = r.collection.UpdateOne(ctx, bson.M{"slug": slug, "buttons.id": buttonID}, bson.M{
		"$inc": bson.M{
			"totalClicks":     1,
			"buttons.$.clicks": 1,
		},
	})
}
