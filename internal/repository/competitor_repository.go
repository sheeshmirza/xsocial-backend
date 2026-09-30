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

type CompetitorRepository struct {
	collection *mongo.Collection
}

func NewCompetitorRepository(db *mongo.Database) *CompetitorRepository {
	repo := &CompetitorRepository{
		collection: db.Collection("competitors"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "brandId", Value: 1}, {Key: "platform", Value: 1}, {Key: "handle", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
	})

	return repo
}

func (r *CompetitorRepository) Create(ctx context.Context, comp *models.Competitor) error {
	now := time.Now()
	comp.CreatedAt = now
	comp.UpdatedAt = now
	comp.LastSyncedAt = now
	res, err := r.collection.InsertOne(ctx, comp)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		comp.ID = oid
	}
	return nil
}

func (r *CompetitorRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Competitor, error) {
	var comp models.Competitor
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&comp)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &comp, nil
}

func (r *CompetitorRepository) FindByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.Competitor, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"brandId": brandID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var list []models.Competitor
	if err := cursor.All(ctx, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *CompetitorRepository) Delete(ctx context.Context, id, brandID bson.ObjectID) (bool, error) {
	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": id, "brandId": brandID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

func (r *CompetitorRepository) Update(ctx context.Context, id bson.ObjectID, update bson.M) error {
	update["updatedAt"] = time.Now()
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	return err
}

func (r *CompetitorRepository) AddSnapshot(ctx context.Context, id bson.ObjectID, snap models.CompetitorSnapshot) error {
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$push": bson.M{"history": snap},
		"$set": bson.M{
			"followers":      snap.Followers,
			"postsCount":     snap.PostsCount,
			"engagementRate": snap.EngagementRate,
			"lastSyncedAt":   time.Now(),
			"updatedAt":      time.Now(),
		},
	})
	return err
}
