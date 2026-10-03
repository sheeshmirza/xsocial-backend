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

type BrandRepository struct {
	brandsCollection  *mongo.Collection
	membersCollection *mongo.Collection
}

func NewBrandRepository(db *mongo.Database) *BrandRepository {
	repo := &BrandRepository{
		brandsCollection:  db.Collection("brands"),
		membersCollection: db.Collection("brand_members"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.brandsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "ownerId", Value: 1}}},
	})
	_, _ = repo.membersCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "brandId", Value: 1}, {Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "userId", Value: 1}}},
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
	})

	return repo
}

func (r *BrandRepository) CreateBrand(ctx context.Context, brand *models.Brand) error {
	return r.CreateBrandWithOwner(ctx, brand, "", "")
}

func (r *BrandRepository) CreateBrandWithOwner(ctx context.Context, brand *models.Brand, ownerEmail, ownerName string) error {
	now := time.Now()
	brand.CreatedAt = now
	brand.UpdatedAt = now
	res, err := r.brandsCollection.InsertOne(ctx, brand)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		brand.ID = oid
	}

	// Add owner as brand member with populated identity
	member := &models.BrandMember{
		BrandID:   brand.ID,
		UserID:    brand.OwnerID,
		Email:     ownerEmail,
		Name:      ownerName,
		Role:      models.BrandRoleOwner,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, _ = r.membersCollection.InsertOne(ctx, member)
	return nil
}

func (r *BrandRepository) FindBrandByID(ctx context.Context, id bson.ObjectID) (*models.Brand, error) {
	var brand models.Brand
	err := r.brandsCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&brand)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &brand, nil
}

func (r *BrandRepository) FindBrandsByUserID(ctx context.Context, userID bson.ObjectID) ([]models.Brand, error) {
	// Find all brand IDs where user is a member
	cursor, err := r.membersCollection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var members []models.BrandMember
	if err := cursor.All(ctx, &members); err != nil {
		return nil, err
	}

	brandIDs := make([]bson.ObjectID, 0, len(members))
	for _, m := range members {
		brandIDs = append(brandIDs, m.BrandID)
	}

	if len(brandIDs) == 0 {
		return []models.Brand{}, nil
	}

	bCursor, err := r.brandsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": brandIDs}}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, err
	}
	defer bCursor.Close(ctx)

	var brands []models.Brand
	if err := bCursor.All(ctx, &brands); err != nil {
		return nil, err
	}
	return brands, nil
}

func (r *BrandRepository) UpdateBrand(ctx context.Context, id bson.ObjectID, update bson.M) (*models.Brand, error) {
	update["updatedAt"] = time.Now()
	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.Brand
	err := r.brandsCollection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": update}, opt).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &updated, nil
}

func (r *BrandRepository) DeleteBrand(ctx context.Context, id bson.ObjectID) (bool, error) {
	res, err := r.brandsCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	_, _ = r.membersCollection.DeleteMany(ctx, bson.M{"brandId": id})
	return res.DeletedCount > 0, nil
}

func (r *BrandRepository) AddMember(ctx context.Context, member *models.BrandMember) error {
	now := time.Now()
	member.CreatedAt = now
	member.UpdatedAt = now
	res, err := r.membersCollection.InsertOne(ctx, member)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		member.ID = oid
	}
	return nil
}

func (r *BrandRepository) FindMembersByBrandID(ctx context.Context, brandID bson.ObjectID) ([]models.BrandMember, error) {
	cursor, err := r.membersCollection.Find(ctx, bson.M{"brandId": brandID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var members []models.BrandMember
	if err := cursor.All(ctx, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (r *BrandRepository) FindMember(ctx context.Context, brandID, userID bson.ObjectID) (*models.BrandMember, error) {
	var member models.BrandMember
	err := r.membersCollection.FindOne(ctx, bson.M{"brandId": brandID, "userId": userID}).Decode(&member)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &member, nil
}

func (r *BrandRepository) RemoveMember(ctx context.Context, brandID, userID bson.ObjectID) (bool, error) {
	res, err := r.membersCollection.DeleteOne(ctx, bson.M{"brandId": brandID, "userId": userID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}
