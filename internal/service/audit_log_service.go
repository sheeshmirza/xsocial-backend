package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"xsocial-backend/internal/models"
	"xsocial-backend/internal/repository"
	"xsocial-backend/internal/utils"
)

type AuditLogService struct {
	repo    repository.IAuditLogRepository
	queue   chan *models.AuditLog
	stopCh  chan struct{}
	closeMu sync.Once
}

func NewAuditLogService(repo repository.IAuditLogRepository) *AuditLogService {
	s := &AuditLogService{
		repo:   repo,
		queue:  make(chan *models.AuditLog, 10000),
		stopCh: make(chan struct{}),
	}
	// Start dedicated asynchronous workers to bound concurrency and avoid unbounded goroutines
	for i := 0; i < 8; i++ {
		go s.startWorker()
	}
	return s
}

func (s *AuditLogService) startWorker() {
	for {
		select {
		case <-s.stopCh:
			return
		case entry, ok := <-s.queue:
			if !ok {
				return
			}
			if s.repo != nil && entry != nil {
				_ = s.repo.Record(context.Background(), entry)
			}
		}
	}
}

func (s *AuditLogService) Log(ctx context.Context, userID *bson.ObjectID, brandID *bson.ObjectID, actor, action, resourceType, resourceID, ip, userAgent string) {
	if s == nil || s.repo == nil {
		return
	}
	logEntry := &models.AuditLog{
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

	// Non-blocking enqueue for sub-microsecond HTTP handler latency
	select {
	case s.queue <- logEntry:
	default:
		// Queue full under extreme traffic spike: process asynchronously via SafeGo
		utils.SafeGo(func() {
			_ = s.repo.Record(context.Background(), logEntry)
		})
	}
}

func (s *AuditLogService) Close() {
	if s == nil {
		return
	}
	s.closeMu.Do(func() {
		close(s.stopCh)
	})
}

func (s *AuditLogService) ListLogs(ctx context.Context, userID *bson.ObjectID, brandID *bson.ObjectID, limit int64) ([]models.AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repo.Find(ctx, userID, brandID, limit)
}
