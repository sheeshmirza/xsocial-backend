package database

import (
	"context"
	"log"
	"time"

	"xsocial-backend/internal/config"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client *redis.Client
}

func ConnectRedis(cfg *config.Config) *RedisClient {
	if cfg.RedisURL == "" {
		log.Println("REDIS_URL is not configured; post queue will use memory fallback")
		return nil
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Printf("Failed to parse REDIS_URL: %v; running without Redis", err)
		return nil
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Failed to connect to Redis at %s: %v; running without Redis", cfg.RedisURL, err)
		return nil
	}
	log.Println("Redis connected successfully")
	return &RedisClient{Client: client}
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	if r == nil || r.Client == nil {
		return "", redis.Nil
	}
	return r.Client.Get(ctx, key).Result()
}

func (r *RedisClient) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Set(ctx, key, value, expiration).Err()
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Del(ctx, keys...).Err()
}
