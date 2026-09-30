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
	PublishPostJobName         = "publish-post"
	ExpirePostPublicationJobName = "expire-post-publication"
	RedisScheduledSet          = "xsocial:scheduled-posts-queue"
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
	Name string `json:"name"`
	Data any    `json:"data"`
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
			ctx := context.Background()
			nowScore := float64(time.Now().UnixMilli())

			// Fetch items whose runAt is <= now
			vals, err := q.redisClient.ZRangeByScore(ctx, RedisScheduledSet, &redis.ZRangeBy{
				Min:    "-inf",
				Max:    fmt.Sprintf("%f", nowScore),
				Offset: 0,
				Count:  10,
			}).Result()

			if err != nil || len(vals) == 0 {
				continue
			}

			for _, rawItem := range vals {
				// Remove item from set
				remCount, remErr := q.redisClient.ZRem(ctx, RedisScheduledSet, rawItem).Result()
				if remErr != nil || remCount == 0 {
					continue
				}

				var envelope struct {
					Name string          `json:"name"`
					Data json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal([]byte(rawItem), &envelope); err == nil {
					go func(n string, d []byte) {
						if err := q.handler(n, d); err != nil {
							log.Printf("[QueueWorker] Job %s failed: %v", n, err)
						}
					}(envelope.Name, envelope.Data)
				}
			}
		}
	}
}

func (q *PostQueue) AddSchedulePublishJob(postID string, runAt time.Time) error {
	_ = q.RemoveSchedulePublishJob(postID)

	data := PublishJobData{PostID: postID}
	env := JobEnvelope{
		Name: PublishPostJobName,
		Data: data,
	}
	bytes, _ := json.Marshal(env)

	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}

	if q.redisClient != nil {
		ctx := context.Background()
		return q.redisClient.ZAdd(ctx, RedisScheduledSet, redis.Z{
			Score:  float64(runAt.UnixMilli()),
			Member: string(bytes),
		}).Err()
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
	if q.redisClient != nil {
		ctx := context.Background()
		vals, err := q.redisClient.ZRange(ctx, RedisScheduledSet, 0, -1).Result()
		if err == nil {
			for _, val := range vals {
				var env struct {
					Name string         `json:"name"`
					Data PublishJobData `json:"data"`
				}
				if err := json.Unmarshal([]byte(val), &env); err == nil {
					if env.Name == PublishPostJobName && env.Data.PostID == postID {
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
	data := ExpiryJobData{
		PostID:         postID,
		UserID:         userID,
		Platform:       platform,
		PlatformPostID: platformPostID,
		SocialHandleID: socialHandleID,
	}
	env := JobEnvelope{
		Name: ExpirePostPublicationJobName,
		Data: data,
	}
	bytes, _ := json.Marshal(env)

	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}

	if q.redisClient != nil {
		ctx := context.Background()
		return q.redisClient.ZAdd(ctx, RedisScheduledSet, redis.Z{
			Score:  float64(runAt.UnixMilli()),
			Member: string(bytes),
		}).Err()
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
		ctx := context.Background()
		vals, err := q.redisClient.ZRange(ctx, RedisScheduledSet, 0, -1).Result()
		if err == nil {
			for _, val := range vals {
				var env struct {
					Name string        `json:"name"`
					Data ExpiryJobData `json:"data"`
				}
				if err := json.Unmarshal([]byte(val), &env); err == nil {
					if env.Name == ExpirePostPublicationJobName && env.Data.PostID == postID {
						_ = q.redisClient.ZRem(ctx, RedisScheduledSet, val).Err()
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
