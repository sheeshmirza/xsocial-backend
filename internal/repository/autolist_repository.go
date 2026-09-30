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

type AutolistRepository struct {
	autolistsCollection *mongo.Collection
	rssFeedsCollection   *mongo.Collection
}

func NewAutolistRepository(db *mongo.Database) *AutolistRepository {
	repo := &AutolistRepository{
		autolistsCollection: db.Collection("autolists"),
		rssFeedsCollection:   db.Collection("rss_feeds"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.autolistsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
		{Keys: bson.D{{Key: "isActive", Value: 1}}},
	})
	_, _ = repo.rssFeedsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
		{Keys: bson.D{{Key: "isActive", Value: 1}}},
	})

	return repo
}

func (r *AutolistRepository) CreateAutolist(ctx context.Context, list *models.Autolist) error {
	now := time.Now()
	list.CreatedAt = now
	list.UpdatedAt = now
	list.IsActive = true
	res, err := r.autolistsCollection.InsertOne(ctx, list)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		list.ID = oid
	}
	return nil
}

func (r *AutolistRepository) FindAutolistByID(ctx context.Context, id bson.ObjectID) (*models.Autolist, error) {
	var list models.Autolist
	err := r.autolistsCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&list)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &list, nil
}

func (r *AutolistRepository) FindAutolistsByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.Autolist, error) {
	cursor, err := r.autolistsCollection.Find(ctx, bson.M{"brandId": brandID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var lists []models.Autolist
	if err := cursor.All(ctx, &lists); err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *AutolistRepository) FindAllActiveAutolists(ctx context.Context) ([]models.Autolist, error) {
	cursor, err := r.autolistsCollection.Find(ctx, bson.M{"isActive": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var lists []models.Autolist
	if err := cursor.All(ctx, &lists); err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *AutolistRepository) UpdateAutolist(ctx context.Context, id bson.ObjectID, update bson.M) (*models.Autolist, error) {
	update["updatedAt"] = time.Now()
	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.Autolist
	err := r.autolistsCollection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": update}, opt).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &updated, nil
}

func (r *AutolistRepository) DeleteAutolist(ctx context.Context, id, brandID bson.ObjectID) (bool, error) {
	res, err := r.autolistsCollection.DeleteOne(ctx, bson.M{"_id": id, "brandId": brandID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// RSS Feeds
func (r *AutolistRepository) CreateRSSFeed(ctx context.Context, feed *models.RSSFeed) error {
	now := time.Now()
	feed.CreatedAt = now
	feed.UpdatedAt = now
	feed.IsActive = true
	res, err := r.rssFeedsCollection.InsertOne(ctx, feed)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		feed.ID = oid
	}
	return nil
}

func (r *AutolistRepository) FindRSSFeedsByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.RSSFeed, error) {
	cursor, err := r.rssFeedsCollection.Find(ctx, bson.M{"brandId": brandID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var feeds []models.RSSFeed
	if err := cursor.All(ctx, &feeds); err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *AutolistRepository) FindActiveRSSFeeds(ctx context.Context) ([]models.RSSFeed, error) {
	cursor, err := r.rssFeedsCollection.Find(ctx, bson.M{"isActive": true})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var feeds []models.RSSFeed
	if err := cursor.All(ctx, &feeds); err != nil {
		return nil, err
	}
	return feeds, nil
}

func (r *AutolistRepository) UpdateRSSFeed(ctx context.Context, id bson.ObjectID, update bson.M) error {
	update["updatedAt"] = time.Now()
	_, err := r.rssFeedsCollection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *AutolistRepository) DeleteRSSFeed(ctx context.Context, id, brandID bson.ObjectID) (bool, error) {
	res, err := r.rssFeedsCollection.DeleteOne(ctx, bson.M{"_id": id, "brandId": brandID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}
