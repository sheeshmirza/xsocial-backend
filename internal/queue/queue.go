package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	amqp "github.com/rabbitmq/amqp091-go"
	"xsocial-backend/internal/database"
	"xsocial-backend/internal/utils"
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
	PostID      string `json:"postId"`
	Traceparent string `json:"traceparent,omitempty"`
	RequestID   string `json:"requestId,omitempty"`
}

type ExpiryJobData struct {
	PostID         string `json:"postId"`
	UserID         string `json:"userId"`
	Platform       string `json:"platform"`
	PlatformPostID string `json:"platformPostId"`
	SocialHandleID string `json:"socialHandleId"`
	Traceparent    string `json:"traceparent,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
}

type JobEnvelope struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Data        any    `json:"data"`
	Retries     int    `json:"retries,omitempty"`
	Traceparent string `json:"traceparent,omitempty"`
	RequestID   string `json:"requestId,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
}

type HandlerFunc func(name string, payload []byte) error

type PostQueue struct {
	redisClient  *redis.Client
	rabbitBroker *RabbitMQBroker
	handler      HandlerFunc
	stopCh       chan struct{}
	closeOnce    sync.Once
	mu           sync.Mutex
	memTimers    map[string]*time.Timer
}

func (q *PostQueue) SetRabbitMQ(broker *RabbitMQBroker) {
	if q == nil || broker == nil {
		return
	}
	q.rabbitBroker = broker
	ctx := context.Background()
	_ = broker.Consume(ctx, QueuePostPublish, 10, func(msgCtx context.Context, delivery amqp.Delivery) error {
		var env JobEnvelope
		if err := json.Unmarshal(delivery.Body, &env); err != nil {
			return err
		}
		dataBytes, _ := json.Marshal(env.Data)
		q.processJob(env.ID, env.Name, dataBytes, env.Retries)
		return nil
	})
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

var popScheduledJobsLua = redis.NewScript(`
local vals = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[2]))
if #vals > 0 then
    for _, val in ipairs(vals) do
        redis.call('ZREM', KEYS[1], val)
    end
end
return vals
`)

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

			// Atomically pop items whose runAt is <= now to prevent duplicate execution across horizontal workers
			res, err := popScheduledJobsLua.Run(ctx, q.redisClient, []string{RedisScheduledSet}, fmt.Sprintf("%f", nowScore), 10).StringSlice()
			if err != nil || len(res) == 0 {
				cancel()
				continue
			}

			for _, rawItem := range res {
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

					if q.rabbitBroker != nil {
						ctxPub, cancelPub := context.WithTimeout(context.Background(), 5*time.Second)
						env := JobEnvelope{
							ID:        envelope.ID,
							Name:      envelope.Name,
							Data:      envelope.Data,
							Retries:   envelope.Retries,
							CreatedAt: envelope.CreatedAt,
						}
						if err := q.rabbitBroker.Publish(ctxPub, ExchangeDirect, RoutingPostPublish, env); err != nil {
							go q.processJob(envelope.ID, envelope.Name, envelope.Data, envelope.Retries)
						}
						cancelPub()
					} else {
						go q.processJob(envelope.ID, envelope.Name, envelope.Data, envelope.Retries)
					}
				}
			}
			cancel()
		}
	}
}

func (q *PostQueue) processJob(jobID, name string, data []byte, retries int) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("queue_worker_panic", "job_id", jobID, "name", name, "panic", fmt.Sprintf("%v", r))
		}
	}()
	if q.handler == nil {
		return
	}
	if err := q.handler(name, data); err != nil {
		slog.Warn("queue_job_failed", "job_id", jobID, "name", name, "attempt", retries+1, "error", err)

		if retries < MaxJobRetries {
			// Retry with exponential backoff: 5s, 20s, 60s
			backoffSecs := []int{5, 20, 60}
			backoffIdx := retries
			if backoffIdx < 0 {
				backoffIdx = 0
			} else if backoffIdx >= len(backoffSecs) {
				backoffIdx = len(backoffSecs) - 1
			}
			backoffSec := backoffSecs[backoffIdx]
			retryAt := time.Now().Add(time.Duration(backoffSec) * time.Second)
			slog.Info("queue_job_requeue", "job_id", jobID, "name", name, "backoff_sec", backoffSec, "attempt", retries+1, "max_retries", MaxJobRetries)

			var parsedData map[string]any
			_ = json.Unmarshal(data, &parsedData)
			var traceparent, reqID string
			if tp, ok := parsedData["traceparent"].(string); ok {
				traceparent = tp
			}
			if rid, ok := parsedData["requestId"].(string); ok {
				reqID = rid
			}
			env := JobEnvelope{
				ID:          jobID,
				Name:        name,
				Data:        parsedData,
				Retries:     retries + 1,
				Traceparent: traceparent,
				RequestID:   reqID,
				CreatedAt:   time.Now().UnixMilli(),
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
			slog.Error("queue_job_dead_letter", "job_id", jobID, "name", name, "max_retries", MaxJobRetries)
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
	return q.AddSchedulePublishJobCtx(context.Background(), postID, runAt)
}

func (q *PostQueue) AddSchedulePublishJobCtx(ctx context.Context, postID string, runAt time.Time) error {
	_ = q.RemoveSchedulePublishJob(postID)

	traceparent := utils.TraceparentFromContext(ctx)
	reqID := utils.RequestIDFromContext(ctx)

	jobID := "publish:" + postID
	data := PublishJobData{
		PostID:      postID,
		Traceparent: traceparent,
		RequestID:   reqID,
	}
	env := JobEnvelope{
		ID:          jobID,
		Name:        PublishPostJobName,
		Data:        data,
		Traceparent: traceparent,
		RequestID:   reqID,
		CreatedAt:   time.Now().UnixMilli(),
	}
	bytes, _ := json.Marshal(env)

	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}

	if q.redisClient != nil {
		cmdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pipe := q.redisClient.TxPipeline()
		pipe.ZAdd(cmdCtx, RedisScheduledSet, redis.Z{
			Score:  float64(runAt.UnixMilli()),
			Member: string(bytes),
		})
		pipe.Set(cmdCtx, RedisJobIndexKey+jobID, string(bytes), 30*24*time.Hour)
		_, err := pipe.Exec(cmdCtx)
		return err
	}

	// Memory fallback
	q.mu.Lock()
	defer q.mu.Unlock()
	timerKey := fmt.Sprintf("publish:%s", postID)
	q.memTimers[timerKey] = time.AfterFunc(delay, func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("queue_memory_publish_job_panic", "post_id", postID, "panic", fmt.Sprintf("%v", r))
			}
			q.mu.Lock()
			delete(q.memTimers, timerKey)
			q.mu.Unlock()
		}()
		dataBytes, _ := json.Marshal(data)
		if q.handler != nil {
			_ = q.handler(PublishPostJobName, dataBytes)
		}
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
	return q.AddPostExpiryJobCtx(context.Background(), postID, userID, platform, platformPostID, socialHandleID, runAt)
}

func (q *PostQueue) AddPostExpiryJobCtx(ctx context.Context, postID, userID, platform, platformPostID, socialHandleID string, runAt time.Time) error {
	traceparent := utils.TraceparentFromContext(ctx)
	reqID := utils.RequestIDFromContext(ctx)

	jobID := fmt.Sprintf("expiry:%s:%s", postID, platformPostID)
	data := ExpiryJobData{
		PostID:         postID,
		UserID:         userID,
		Platform:       platform,
		PlatformPostID: platformPostID,
		SocialHandleID: socialHandleID,
		Traceparent:    traceparent,
		RequestID:      reqID,
	}
	env := JobEnvelope{
		ID:          jobID,
		Name:        ExpirePostPublicationJobName,
		Data:        data,
		Traceparent: traceparent,
		RequestID:   reqID,
		CreatedAt:   time.Now().UnixMilli(),
	}
	bytes, _ := json.Marshal(env)

	delay := time.Until(runAt)
	if delay < 0 {
		delay = 0
	}

	if q.redisClient != nil {
		cmdCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pipe := q.redisClient.TxPipeline()
		pipe.ZAdd(cmdCtx, RedisScheduledSet, redis.Z{
			Score:  float64(runAt.UnixMilli()),
			Member: string(bytes),
		})
		pipe.Set(cmdCtx, RedisJobIndexKey+jobID, string(bytes), 30*24*time.Hour)
		_, err := pipe.Exec(cmdCtx)
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	timerKey := fmt.Sprintf("expiry:%s:%s", postID, platformPostID)
	q.memTimers[timerKey] = time.AfterFunc(delay, func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("queue_memory_expiry_job_panic", "post_id", postID, "panic", fmt.Sprintf("%v", r))
			}
			q.mu.Lock()
			delete(q.memTimers, timerKey)
			q.mu.Unlock()
		}()
		dataBytes, _ := json.Marshal(data)
		if q.handler != nil {
			_ = q.handler(ExpirePostPublicationJobName, dataBytes)
		}
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
			if timer != nil {
				timer.Stop()
			}
			delete(q.memTimers, k)
		}
	}
	q.mu.Unlock()

	return nil
}

func (q *PostQueue) Close() {
	q.closeOnce.Do(func() {
		close(q.stopCh)
		q.mu.Lock()
		for k, timer := range q.memTimers {
			if timer != nil {
				timer.Stop()
			}
			delete(q.memTimers, k)
		}
		q.mu.Unlock()
	})
}
