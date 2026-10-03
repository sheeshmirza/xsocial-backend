package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
)

type AuditLogService struct {
	repo repository.IAuditLogRepository
}

func NewAuditLogService(repo repository.IAuditLogRepository) *AuditLogService {
	return &AuditLogService{
		repo: repo,
	}
}

func (s *AuditLogService) Log(ctx context.Context, userID *bson.ObjectID, brandID *bson.ObjectID, actor, action, resourceType, resourceID, ip, userAgent string) {
	go func() {
		log := &models.AuditLog{
			UserID:       userID,
			BrandID:      brandID,
			Actor:        actor,
			Action:       action,
			ResourceType: resourceType,
			ResourceID:   resourceID,
			IPAddress:    ip,
			UserAgent:    userAgent,
			CreatedAt:    time.Now(),
		}
		s.repo.Record(context.Background(), log)
	}()
}

func (s *AuditLogService) ListLogs(ctx context.Context, userID *bson.ObjectID, brandID *bson.ObjectID, limit int64) ([]models.AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.Find(ctx, userID, brandID, limit)
}
