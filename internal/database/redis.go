package database

import (
	"context"
	"log/slog"
	"time"

	"xsocial-backend/internal/config"
	"xsocial-backend/internal/utils"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	Client *redis.Client
}

func ConnectRedis(cfg *config.Config) *RedisClient {
	if cfg.RedisURL == "" {
		slog.Info("redis_not_configured_using_memory_fallback")
		return nil
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Warn("redis_parse_url_failed", "error", err)
		return nil
	}
	opt.PoolSize = 250
	opt.MinIdleConns = 25
	opt.ConnMaxIdleTime = 5 * time.Minute
	opt.DialTimeout = 5 * time.Second
	opt.ReadTimeout = 2 * time.Second
	opt.WriteTimeout = 2 * time.Second

	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("redis_connect_failed", "url", utils.MaskURL(cfg.RedisURL), "error", err)
		return nil
	}
	slog.Info("redis_connected_successfully")
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

func (r *RedisClient) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	if r == nil || r.Client == nil {
		return false, nil
	}
	return r.Client.SetNX(ctx, key, value, expiration).Result()
}

func (r *RedisClient) Del(ctx context.Context, keys ...string) error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Del(ctx, keys...).Err()
}

func (r *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	if r == nil || r.Client == nil {
		return 0, redis.Nil
	}
	return r.Client.Incr(ctx, key).Result()
}

func (r *RedisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Expire(ctx, key, expiration).Err()
}

func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	if r == nil || r.Client == nil {
		return 0, redis.Nil
	}
	return r.Client.TTL(ctx, key).Result()
}

func (r *RedisClient) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	if r == nil || r.Client == nil {
		return nil, redis.Nil
	}
	return r.Client.Eval(ctx, script, keys, args...).Result()
}

var rateLimitLua = redis.NewScript(`
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
if current > tonumber(ARGV[2]) then
    return 0
end
return 1
`)

func (r *RedisClient) AllowRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	if r == nil || r.Client == nil {
		return false, redis.Nil
	}
	res, err := rateLimitLua.Run(ctx, r.Client, []string{key}, window.Milliseconds(), limit).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}
