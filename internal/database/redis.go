package database

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"xsocial-backend/internal/config"
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
