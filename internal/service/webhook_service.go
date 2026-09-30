package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type WebhookService struct {
	repo       *repository.WebhookRepository
	httpClient *http.Client
}

func NewWebhookService(repo *repository.WebhookRepository) *WebhookService {
	return &WebhookService{
		repo: repo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *WebhookService) CreateWebhook(ctx context.Context, userID bson.ObjectID, req models.CreateWebhookRequest) (*models.Webhook, error) {
	var brandID *bson.ObjectID
	if req.BrandID != "" {
		if bid, err := bson.ObjectIDFromHex(req.BrandID); err == nil {
			brandID = &bid
		}
	}

	secret := utils.GenerateRandomToken(32)
	wh := &models.Webhook{
		UserID:  userID,
		BrandID: brandID,
		URL:     req.URL,
		Events:  req.Events,
		Secret:  secret,
	}

	if err := s.repo.Create(ctx, wh); err != nil {
		return nil, utils.InternalServerError("Failed to create webhook")
	}
	return wh, nil
}

func (s *WebhookService) ListWebhooks(ctx context.Context, userID bson.ObjectID, brandID *bson.ObjectID) ([]models.Webhook, error) {
	return s.repo.FindByUserOrBrand(ctx, userID, brandID)
}

func (s *WebhookService) DeleteWebhook(ctx context.Context, id, userID bson.ObjectID) error {
	deleted, err := s.repo.Delete(ctx, id, userID)
	if err != nil {
		return utils.InternalServerError("Failed to delete webhook")
	}
	if !deleted {
		return utils.NotFound("Webhook not found")
	}
	return nil
}

func (s *WebhookService) DispatchEvent(ctx context.Context, eventType string, payload any, brandID *bson.ObjectID) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var zeroID bson.ObjectID
		webhooks, err := s.repo.FindByUserOrBrand(bgCtx, zeroID, brandID)
		if err != nil || len(webhooks) == 0 {
			return
		}

		bodyBytes, err := json.Marshal(map[string]any{
			"event":     eventType,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"data":      payload,
		})
		if err != nil {
			return
		}

		for _, wh := range webhooks {
			if wh.Status != "active" {
				continue
			}
			matched := false
			for _, e := range wh.Events {
				if e == "*" || e == eventType {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}

			// Sign payload with HMAC-SHA256
			h := hmac.New(sha256.New, []byte(wh.Secret))
			h.Write(bodyBytes)
			sig := hex.EncodeToString(h.Sum(nil))

			req, rErr := http.NewRequestWithContext(bgCtx, "POST", wh.URL, bytes.NewReader(bodyBytes))
			if rErr == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-XSocial-Signature", sig)
				req.Header.Set("User-Agent", "XSocial-Webhook-Dispatcher/1.0")
				_, _ = s.httpClient.Do(req)
			}
		}
	}()
}
