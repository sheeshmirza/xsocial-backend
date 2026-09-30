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

type InboxRepository struct {
	conversationsCollection *mongo.Collection
	messagesCollection      *mongo.Collection
	cannedRepliesCollection *mongo.Collection
}

func NewInboxRepository(db *mongo.Database) *InboxRepository {
	repo := &InboxRepository{
		conversationsCollection: db.Collection("inbox_conversations"),
		messagesCollection:      db.Collection("inbox_messages"),
		cannedRepliesCollection: db.Collection("inbox_canned_replies"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, _ = repo.conversationsCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "brandId", Value: 1}, {Key: "lastMessageAt", Value: -1}}},
		{Keys: bson.D{{Key: "platform", Value: 1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
	})
	_, _ = repo.messagesCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "conversationId", Value: 1}, {Key: "sentAt", Value: 1}}},
	})
	_, _ = repo.cannedRepliesCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "brandId", Value: 1}}},
	})

	return repo
}

func (r *InboxRepository) CreateConversation(ctx context.Context, conv *models.InboxConversation) error {
	now := time.Now()
	conv.CreatedAt = now
	conv.UpdatedAt = now
	conv.LastMessageAt = now
	if conv.Status == "" {
		conv.Status = models.InboxStatusOpen
	}
	res, err := r.conversationsCollection.InsertOne(ctx, conv)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		conv.ID = oid
	}
	return nil
}

func (r *InboxRepository) FindConversationByID(ctx context.Context, id bson.ObjectID) (*models.InboxConversation, error) {
	var conv models.InboxConversation
	err := r.conversationsCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&conv)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &conv, nil
}

func (r *InboxRepository) FindConversationsByBrand(ctx context.Context, brandID bson.ObjectID, status string, platform string, limit int64, skip int64) ([]models.InboxConversation, int64, error) {
	filter := bson.M{"brandId": brandID}
	if status != "" {
		filter["status"] = status
	}
	if platform != "" {
		filter["platform"] = platform
	}

	total, err := r.conversationsCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().SetSort(bson.D{{Key: "lastMessageAt", Value: -1}}).SetLimit(limit).SetSkip(skip)
	cursor, err := r.conversationsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var convs []models.InboxConversation
	if err := cursor.All(ctx, &convs); err != nil {
		return nil, 0, err
	}
	return convs, total, nil
}

func (r *InboxRepository) UpdateConversation(ctx context.Context, id bson.ObjectID, update bson.M) (*models.InboxConversation, error) {
	update["updatedAt"] = time.Now()
	opt := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var updated models.InboxConversation
	err := r.conversationsCollection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": update}, opt).Decode(&updated)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}
	return &updated, nil
}

func (r *InboxRepository) AddMessage(ctx context.Context, msg *models.InboxMessage) error {
	now := time.Now()
	msg.SentAt = now
	res, err := r.messagesCollection.InsertOne(ctx, msg)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		msg.ID = oid
	}

	// Update conversation's lastMessage
	_, _ = r.conversationsCollection.UpdateOne(ctx, bson.M{"_id": msg.ConversationID}, bson.M{
		"$set": bson.M{
			"lastMessageText": msg.MessageText,
			"lastMessageAt":   now,
			"updatedAt":       now,
		},
	})
	return nil
}

func (r *InboxRepository) FindMessagesByConversationID(ctx context.Context, conversationID bson.ObjectID) ([]models.InboxMessage, error) {
	cursor, err := r.messagesCollection.Find(ctx, bson.M{"conversationId": conversationID}, options.Find().SetSort(bson.D{{Key: "sentAt", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var messages []models.InboxMessage
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

func (r *InboxRepository) CreateCannedReply(ctx context.Context, reply *models.CannedReply) error {
	reply.CreatedAt = time.Now()
	res, err := r.cannedRepliesCollection.InsertOne(ctx, reply)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		reply.ID = oid
	}
	return nil
}

func (r *InboxRepository) FindCannedRepliesByBrand(ctx context.Context, brandID bson.ObjectID) ([]models.CannedReply, error) {
	cursor, err := r.cannedRepliesCollection.Find(ctx, bson.M{"brandId": brandID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var replies []models.CannedReply
	if err := cursor.All(ctx, &replies); err != nil {
		return nil, err
	}
	return replies, nil
}

func (r *InboxRepository) DeleteCannedReply(ctx context.Context, id, brandID bson.ObjectID) (bool, error) {
	res, err := r.cannedRepliesCollection.DeleteOne(ctx, bson.M{"_id": id, "brandId": brandID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}
