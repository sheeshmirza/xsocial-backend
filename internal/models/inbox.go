package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type InboxStatus string

const (
	InboxStatusOpen     InboxStatus = "open"
	InboxStatusResolved InboxStatus = "resolved"
	InboxStatusArchived InboxStatus = "archived"
)

type InboxSentiment string

const (
	SentimentPositive InboxSentiment = "positive"
	SentimentNeutral  InboxSentiment = "neutral"
	SentimentNegative InboxSentiment = "negative"
)

type InboxConversation struct {
	ID               bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	BrandID          bson.ObjectID   `bson:"brandId" json:"brandId"`
	Platform         string          `bson:"platform" json:"platform"` // instagram, facebook, linkedin, tiktok, twitter, google_business
	SocialHandleID   bson.ObjectID   `bson:"socialHandleId" json:"socialHandleId"`
	ExternalThreadID string          `bson:"externalThreadID" json:"externalThreadId"`
	SenderName       string          `bson:"senderName" json:"senderName"`
	SenderAvatar     string          `bson:"senderAvatar" json:"senderAvatar"`
	SenderHandle     string          `bson:"senderHandle" json:"senderHandle"`
	LastMessageText  string          `bson:"lastMessageText" json:"lastMessageText"`
	LastMessageAt    time.Time       `bson:"lastMessageAt" json:"lastMessageAt"`
	Status           InboxStatus     `bson:"status" json:"status"`
	Unread           bool            `bson:"unread" json:"unread"`
	AssignedTo       *bson.ObjectID  `bson:"assignedTo,omitempty" json:"assignedTo,omitempty"`
	Tags             []string        `bson:"tags" json:"tags"`
	Sentiment        InboxSentiment  `bson:"sentiment" json:"sentiment"`
	Rating           *int            `bson:"rating,omitempty" json:"rating,omitempty"` // For Google Business reviews (1-5)
	CreatedAt        time.Time       `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time       `bson:"updatedAt" json:"updatedAt"`
}

type InboxMessage struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ConversationID bson.ObjectID `bson:"conversationId" json:"conversationId"`
	SenderType     string        `bson:"senderType" json:"senderType"` // contact, user, bot
	SenderID       string        `bson:"senderId" json:"senderId"`
	SenderName     string        `bson:"senderName" json:"senderName"`
	MessageText    string        `bson:"messageText" json:"messageText"`
	MediaURLs      []string      `bson:"mediaUrls" json:"mediaUrls"`
	SentAt         time.Time     `bson:"sentAt" json:"sentAt"`
}

type CannedReply struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	BrandID   bson.ObjectID `bson:"brandId" json:"brandId"`
	Shortcut  string        `bson:"shortcut" json:"shortcut"` // e.g. "/thanks"
	Title     string        `bson:"title" json:"title"`
	Content   string        `bson:"content" json:"content"`
	CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
}

type ReplyInboxRequest struct {
	MessageText string   `json:"messageText" binding:"required"`
	MediaURLs   []string `json:"mediaUrls"`
}

type UpdateInboxStatusRequest struct {
	Status     InboxStatus    `json:"status"`
	Unread     *bool          `json:"unread"`
	AssignedTo *string        `json:"assignedTo"`
	Tags       []string       `json:"tags"`
	Sentiment  InboxSentiment `json:"sentiment"`
}
