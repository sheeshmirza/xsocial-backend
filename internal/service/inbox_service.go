package service

import (
	"context"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type InboxService struct {
	repo       *repository.InboxRepository
	brandRepo  *repository.BrandRepository
	socialRepo *repository.SocialHandleRepository
}

func NewInboxService(repo *repository.InboxRepository, brandRepo *repository.BrandRepository, socialRepo *repository.SocialHandleRepository) *InboxService {
	return &InboxService{
		repo:       repo,
		brandRepo:  brandRepo,
		socialRepo: socialRepo,
	}
}

func (s *InboxService) ListConversations(ctx context.Context, brandID, userID bson.ObjectID, status, platform string, page, limit int64) ([]models.InboxConversation, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	skip := (page - 1) * limit

	return s.repo.FindConversationsByBrand(ctx, brandID, status, platform, limit, skip)
}

func (s *InboxService) GetConversation(ctx context.Context, id, brandID bson.ObjectID) (*models.InboxConversation, []models.InboxMessage, error) {
	conv, err := s.repo.FindConversationByID(ctx, id)
	if err != nil {
		return nil, nil, utils.InternalServerError("Failed to get conversation")
	}
	if conv == nil || conv.BrandID != brandID {
		return nil, nil, utils.NotFound("Conversation not found")
	}

	messages, err := s.repo.FindMessagesByConversationID(ctx, id)
	if err != nil {
		return nil, nil, utils.InternalServerError("Failed to get messages")
	}

	// Mark as read
	if conv.Unread {
		_, _ = s.repo.UpdateConversation(ctx, id, bson.M{"unread": false})
		conv.Unread = false
	}

	return conv, messages, nil
}

func (s *InboxService) ReplyToConversation(ctx context.Context, id, brandID, userID bson.ObjectID, userName string, req models.ReplyInboxRequest) (*models.InboxMessage, error) {
	conv, err := s.repo.FindConversationByID(ctx, id)
	if err != nil || conv == nil || conv.BrandID != brandID {
		return nil, utils.NotFound("Conversation not found")
	}

	msg := &models.InboxMessage{
		ConversationID: id,
		SenderType:     "user",
		SenderID:       userID.Hex(),
		SenderName:     userName,
		MessageText:    req.MessageText,
		MediaURLs:      req.MediaURLs,
	}

	if err := s.repo.AddMessage(ctx, msg); err != nil {
		return nil, utils.InternalServerError("Failed to send reply")
	}

	return msg, nil
}

func (s *InboxService) UpdateConversationStatus(ctx context.Context, id, brandID bson.ObjectID, req models.UpdateInboxStatusRequest) (*models.InboxConversation, error) {
	conv, err := s.repo.FindConversationByID(ctx, id)
	if err != nil || conv == nil || conv.BrandID != brandID {
		return nil, utils.NotFound("Conversation not found")
	}

	update := bson.M{}
	if req.Status != "" {
		update["status"] = req.Status
	}
	if req.Unread != nil {
		update["unread"] = *req.Unread
	}
	if req.Sentiment != "" {
		update["sentiment"] = req.Sentiment
	}
	if len(req.Tags) > 0 {
		update["tags"] = req.Tags
	}
	if req.AssignedTo != nil {
		if *req.AssignedTo == "" {
			update["assignedTo"] = nil
		} else if aid, err := bson.ObjectIDFromHex(*req.AssignedTo); err == nil {
			update["assignedTo"] = aid
		}
	}

	if len(update) == 0 {
		return conv, nil
	}

	return s.repo.UpdateConversation(ctx, id, update)
}

func (s *InboxService) CreateCannedReply(ctx context.Context, brandID bson.ObjectID, shortcut, title, content string) (*models.CannedReply, error) {
	if !strings.HasPrefix(shortcut, "/") {
		shortcut = "/" + shortcut
	}
	reply := &models.CannedReply{
		BrandID:  brandID,
		Shortcut: shortcut,
		Title:    title,
		Content:  content,
	}
	if err := s.repo.CreateCannedReply(ctx, reply); err != nil {
		return nil, utils.InternalServerError("Failed to create canned reply")
	}
	return reply, nil
}

func (s *InboxService) ListCannedReplies(ctx context.Context, brandID bson.ObjectID) ([]models.CannedReply, error) {
	return s.repo.FindCannedRepliesByBrand(ctx, brandID)
}

func (s *InboxService) DeleteCannedReply(ctx context.Context, id, brandID bson.ObjectID) error {
	deleted, err := s.repo.DeleteCannedReply(ctx, id, brandID)
	if err != nil {
		return utils.InternalServerError("Failed to delete canned reply")
	}
	if !deleted {
		return utils.NotFound("Canned reply not found")
	}
	return nil
}

// IngestExternalMessage simulates / ingests incoming webhook message from Meta / LinkedIn / etc.
func (s *InboxService) IngestIncomingMessage(ctx context.Context, brandID bson.ObjectID, platform, threadID, senderName, senderAvatar, senderHandle, text string, rating *int) error {
	sentiment := analyzeSentiment(text)

	// Find existing conversation or create new
	convs, _, err := s.repo.FindConversationsByBrand(ctx, brandID, "", platform, 1, 0)
	var conv *models.InboxConversation
	if err == nil {
		for i := range convs {
			if convs[i].ExternalThreadID == threadID {
				conv = &convs[i]
				break
			}
		}
	}

	if conv == nil {
		conv = &models.InboxConversation{
			BrandID:          brandID,
			Platform:         platform,
			ExternalThreadID: threadID,
			SenderName:       senderName,
			SenderAvatar:     senderAvatar,
			SenderHandle:     senderHandle,
			LastMessageText:  text,
			Status:           models.InboxStatusOpen,
			Unread:           true,
			Sentiment:        sentiment,
			Rating:           rating,
		}
		if err := s.repo.CreateConversation(ctx, conv); err != nil {
			return err
		}
	} else {
		_, _ = s.repo.UpdateConversation(ctx, conv.ID, bson.M{
			"lastMessageText": text,
			"unread":          true,
			"status":          models.InboxStatusOpen,
			"sentiment":       sentiment,
		})
	}

	msg := &models.InboxMessage{
		ConversationID: conv.ID,
		SenderType:     "contact",
		SenderID:       senderHandle,
		SenderName:     senderName,
		MessageText:    text,
	}
	return s.repo.AddMessage(ctx, msg)
}

func analyzeSentiment(text string) models.InboxSentiment {
	lower := strings.ToLower(text)
	positiveWords := []string{"great", "love", "awesome", "good", "thank", "happy", "best", "excellent", "amazing"}
	negativeWords := []string{"bad", "worst", "hate", "terrible", "awful", "scam", "broken", "angry", "disappointed", "refund"}

	for _, w := range positiveWords {
		if strings.Contains(lower, w) {
			return models.SentimentPositive
		}
	}
	for _, w := range negativeWords {
		if strings.Contains(lower, w) {
			return models.SentimentNegative
		}
	}
	return models.SentimentNeutral
}
