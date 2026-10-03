package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"xsocial-backend/internal/database"
)

const (
	PublishPostJobName           = "publish-post"
	ExpirePostPublicationJobName = "expire-post-publication"
	RedisScheduledSet            = "xsocial:scheduled-posts-queue"
	RedisJobIndexKey             = "xsocial:job-index:"     // xsocial:job-index:<jobID> -> member payload
	RedisDeadLetterSet           = "xsocial:dead-letter-queue"
	MaxJobRetries                = 3
)

type PublishJobData struct {
	PostID string `json:"postId"`
}

type ExpiryJobData struct {
	PostID         string `json:"postId"`
	UserID         string `json:"userId"`
	Platform       string `json:"platform"`
	PlatformPostID string `json:"platformPostId"`
	SocialHandleID string `json:"socialHandleId"`
}

type JobEnvelope struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Data      any    `json:"data"`
	Retries   int    `json:"retries,omitempty"`
	CreatedAt int64  `json:"createdAt"`
}

type HandlerFunc func(name string, payload []byte) error

type PostQueue struct {
	redisClient *redis.Client
	handler     HandlerFunc
	stopCh      chan struct{}
	mu          sync.Mutex
	memTimers   map[string]*time.Timer
}

func NewPostQueue(redisDb *database.RedisClient, handler HandlerFunc) *PostQueue {
	var rClient *redis.Client
	if redisDb != nil {
		rClient = redisDb.Client
	}

	pq := &PostQueue{
		redisClient: rClient,
		handler:     handler,
		stopCh:      make(chan struct{}),
		memTimers:   make(map[string]*time.Timer),
	}

	if rClient != nil {
		go pq.startRedisWorker()
	}

	return pq
}

func (q *PostQueue) startRedisWorker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-q.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			nowScore := float64(time.Now().UnixMilli())

			// Fetch items whose runAt is <= now
			vals, err := q.redisClient.ZRangeByScore(ctx, RedisScheduledSet, &redis.ZRangeBy{
				Min:    "-inf",
				Max:    fmt.Sprintf("%f", nowScore),
				Offset: 0,
				Count:  10,
			}).Result()

			if err != nil || len(vals) == 0 {
				cancel()
				continue
			}

			for _, rawItem := range vals {
				// Atomically pop item from scheduled set to prevent duplicate execution across workers
				remCount, remErr := q.redisClient.ZRem(ctx, RedisScheduledSet, rawItem).Result()
				if remErr != nil || remCount == 0 {
					continue
				}

				var envelope struct {
					ID        string          `json:"id"`
					Name      string          `json:"name"`
					Data      json.RawMessage `json:"data"`
					Retries   int             `json:"retries"`
					CreatedAt int64           `json:"createdAt"`
				}
				if err := json.Unmarshal([]byte(rawItem), &envelope); err == nil {
					// Clean up the index key
					if envelope.ID != "" {
						_ = q.redisClient.Del(ctx, RedisJobIndexKey+envelope.ID).Err()
					}

					go q.processJob(envelope.ID, envelope.Name, envelope.Data, envelope.Retries)
				}
			}
			cancel()
		}
	}
}

func (q *PostQueue) processJob(jobID, name string, data []byte, retries int) {
	if err := q.handler(name, data); err != nil {
		log.Printf("[QueueWorker] Job %s (id: %s, attempt: %d) failed: %v", name, jobID, retries+1, err)

		if retries < MaxJobRetries {
			// Retry with exponential backoff: 5s, 20s, 60s
			backoffSec := []int{5, 20, 60}[retries]
			retryAt := time.Now().Add(time.Duration(backoffSec) * time.Second)
			log.Printf("[QueueWorker] Requeuing job %s (id: %s) in %d seconds (attempt %d/%d)", name, jobID, backoffSec, retries+1, MaxJobRetries)

			var parsedData any
			_ = json.Unmarshal(data, &parsedData)
			env := JobEnvelope{
				ID:        jobID,
				Name:      name,
				Data:      parsedData,
				Retries:   retries + 1,
				CreatedAt: time.Now().UnixMilli(),
			}
			bytes, _ := json.Marshal(env)

			if q.redisClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = q.redisClient.ZAdd(ctx, RedisScheduledSet, redis.Z{
					Score:  float64(retryAt.UnixMilli()),
					Member: string(bytes),
				}).Err()
				if jobID != "" {
					_ = q.redisClient.Set(ctx, RedisJobIndexKey+jobID, string(bytes), 24*time.Hour).Err()
				}
			}
		} else {
			// Dead letter queue
			log.Printf("[QueueWorker] Job %s (id: %s) exceeded max retries (%d). Sent to dead-letter queue.", name, jobID, MaxJobRetries)
			if q.redisClient != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = q.redisClient.ZAdd(ctx, RedisDeadLetterSet, redis.Z{
					Score:  float64(time.Now().UnixMilli()),
					Member: fmt.Sprintf(`{"id":%q,"name":%q,"retries":%d,"failedAt":%d}`, jobID, name, retries+1, time.Now().UnixMilli()),
				}).Err()
			}
		}
	}
}

func (q *PostQueue) AddSchedulePublishJob(postID string, runAt time.Time) error {
	_ = q.RemoveSchedulePublishJob(postID)

	jobID := "publish:" + postID
	data := PublishJobData{PostID: postID}
	env := JobEnvelope{
		ID:        jobID,
		Name:      PublishPostJobName,
		Data:      data,
		CreatedAt: time.Now().UnixMilli(),
	}
	bytes, _ := json.Marshal(env)

	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}

	if q.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pipe := q.redisClient.TxPipeline()
		pipe.ZAdd(ctx, RedisScheduledSet, redis.Z{
			Score:  float64(runAt.UnixMilli()),
			Member: string(bytes),
		})
		pipe.Set(ctx, RedisJobIndexKey+jobID, string(bytes), 30*24*time.Hour)
		_, err := pipe.Exec(ctx)
		return err
	}

	// Memory fallback
	q.mu.Lock()
	defer q.mu.Unlock()
	timerKey := fmt.Sprintf("publish:%s", postID)
	q.memTimers[timerKey] = time.AfterFunc(delay, func() {
		dataBytes, _ := json.Marshal(data)
		_ = q.handler(PublishPostJobName, dataBytes)
		q.mu.Lock()
		delete(q.memTimers, timerKey)
		q.mu.Unlock()
	})

	return nil
}

func (q *PostQueue) RemoveSchedulePublishJob(postID string) error {
	jobID := "publish:" + postID

	if q.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// O(1) direct removal via tracked job index
		if rawMember, err := q.redisClient.Get(ctx, RedisJobIndexKey+jobID).Result(); err == nil && rawMember != "" {
			pipe := q.redisClient.TxPipeline()
			pipe.ZRem(ctx, RedisScheduledSet, rawMember)
			pipe.Del(ctx, RedisJobIndexKey+jobID)
			_, _ = pipe.Exec(ctx)
		} else {
			// Fallback: search sorted set if index key missing
			vals, _ := q.redisClient.ZRange(ctx, RedisScheduledSet, 0, -1).Result()
			for _, val := range vals {
				var env struct {
					ID   string         `json:"id"`
					Name string         `json:"name"`
					Data PublishJobData `json:"data"`
				}
				if json.Unmarshal([]byte(val), &env) == nil {
					if (env.ID == jobID) || (env.Name == PublishPostJobName && env.Data.PostID == postID) {
						_ = q.redisClient.ZRem(ctx, RedisScheduledSet, val).Err()
					}
				}
			}
		}
	}

	q.mu.Lock()
	timerKey := fmt.Sprintf("publish:%s", postID)
	if timer, ok := q.memTimers[timerKey]; ok {
		timer.Stop()
		delete(q.memTimers, timerKey)
	}
	q.mu.Unlock()

	return nil
}

func (q *PostQueue) AddPostExpiryJob(postID, userID, platform, platformPostID, socialHandleID string, runAt time.Time) error {
	jobID := fmt.Sprintf("expiry:%s:%s", postID, platformPostID)
	data := ExpiryJobData{
		PostID:         postID,
		UserID:         userID,
		Platform:       platform,
		PlatformPostID: platformPostID,
		SocialHandleID: socialHandleID,
	}
	env := JobEnvelope{
		ID:        jobID,
		Name:      ExpirePostPublicationJobName,
		Data:      data,
		CreatedAt: time.Now().UnixMilli(),
	}
	bytes, _ := json.Marshal(env)

	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}

	if q.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pipe := q.redisClient.TxPipeline()
		pipe.ZAdd(ctx, RedisScheduledSet, redis.Z{
			Score:  float64(runAt.UnixMilli()),
			Member: string(bytes),
		})
		pipe.Set(ctx, RedisJobIndexKey+jobID, string(bytes), 30*24*time.Hour)
		_, err := pipe.Exec(ctx)
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	timerKey := fmt.Sprintf("expiry:%s:%s", postID, platformPostID)
	q.memTimers[timerKey] = time.AfterFunc(delay, func() {
		dataBytes, _ := json.Marshal(data)
		_ = q.handler(ExpirePostPublicationJobName, dataBytes)
		q.mu.Lock()
		delete(q.memTimers, timerKey)
		q.mu.Unlock()
	})

	return nil
}

func (q *PostQueue) RemovePostExpiryJobs(postID string) error {
	if q.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		vals, err := q.redisClient.ZRange(ctx, RedisScheduledSet, 0, -1).Result()
		if err == nil {
			for _, val := range vals {
				var env struct {
					ID   string        `json:"id"`
					Name string        `json:"name"`
					Data ExpiryJobData `json:"data"`
				}
				if err := json.Unmarshal([]byte(val), &env); err == nil {
					if env.Name == ExpirePostPublicationJobName && env.Data.PostID == postID {
						_ = q.redisClient.ZRem(ctx, RedisScheduledSet, val).Err()
						if env.ID != "" {
							_ = q.redisClient.Del(ctx, RedisJobIndexKey+env.ID).Err()
						}
					}
				}
			}
		}
	}

	q.mu.Lock()
	for k, timer := range q.memTimers {
		if len(k) > len("expiry:"+postID) && k[:len("expiry:"+postID)] == "expiry:"+postID {
			timer.Stop()
			delete(q.memTimers, k)
		}
	}
	q.mu.Unlock()

	return nil
}

func (q *PostQueue) Close() {
	close(q.stopCh)
}
