package database

import (
	"context"
	"hash/fnv"
	"sync"
	"time"
)

// memCacheItem stores a cached value with its expiration timestamp.
type memCacheItem struct {
	value     string
	expiresAt time.Time
}

// memCacheShard is a thread-safe partition of the in-memory cache to reduce lock contention.
type memCacheShard struct {
	mu    sync.RWMutex
	items map[string]memCacheItem
}

// MemoryCache is a high-throughput, sharded in-process cache with TTL auto-eviction.
type MemoryCache struct {
	shards    []*memCacheShard
	numShards int
	stopCh    chan struct{}
	closeOnce sync.Once
}

// NewMemoryCache creates an L1 in-memory cache with sharded locking.
func NewMemoryCache(numShards int, cleanupInterval time.Duration) *MemoryCache {
	if numShards <= 0 {
		numShards = 32
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 30 * time.Second
	}

	mc := &MemoryCache{
		shards:    make([]*memCacheShard, numShards),
		numShards: numShards,
		stopCh:    make(chan struct{}),
	}
	for i := 0; i < numShards; i++ {
		mc.shards[i] = &memCacheShard{
			items: make(map[string]memCacheItem),
		}
	}

	go mc.startEvictionWorker(cleanupInterval)
	return mc
}

func (m *MemoryCache) getShard(key string) *memCacheShard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	idx := int(h.Sum32()) % m.numShards
	if idx < 0 {
		idx = -idx
	}
	return m.shards[idx]
}

// Get returns the cached value if present and not expired.
func (m *MemoryCache) Get(key string) (string, bool) {
	shard := m.getShard(key)
	shard.mu.RLock()
	item, found := shard.items[key]
	shard.mu.RUnlock()

	if !found {
		return "", false
	}
	if time.Now().After(item.expiresAt) {
		return "", false
	}
	return item.value, true
}

// Set stores a key-value pair with the specified TTL.
func (m *MemoryCache) Set(key, value string, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	shard := m.getShard(key)
	shard.mu.Lock()
	// Bound shard size to 5,000 items (max 160,000 items across 32 shards) to avoid OOM
	if len(shard.items) > 5000 {
		now := time.Now()
		for k, v := range shard.items {
			if now.After(v.expiresAt) {
				delete(shard.items, k)
			}
		}
	}
	shard.items[key] = memCacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
	shard.mu.Unlock()
}

// Delete removes a key from the memory cache.
func (m *MemoryCache) Delete(key string) {
	shard := m.getShard(key)
	shard.mu.Lock()
	delete(shard.items, key)
	shard.mu.Unlock()
}

// Close gracefully stops the memory eviction worker.
func (m *MemoryCache) Close() {
	m.closeOnce.Do(func() {
		close(m.stopCh)
	})
}

func (m *MemoryCache) startEvictionWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			now := time.Now()
			for _, shard := range m.shards {
				shard.mu.Lock()
				for k, v := range shard.items {
					if now.After(v.expiresAt) {
						delete(shard.items, k)
					}
				}
				shard.mu.Unlock()
			}
		}
	}
}

// MultiTierCache combines L1 in-memory micro-caching with L2 Redis caching.
type MultiTierCache struct {
	l1    *MemoryCache
	redis *RedisClient
	l1TTL time.Duration
}

// NewMultiTierCache constructs a multi-tier cache.
func NewMultiTierCache(redis *RedisClient, l1TTL time.Duration) *MultiTierCache {
	if l1TTL <= 0 {
		l1TTL = 30 * time.Second // 30-second L1 micro-cache default
	}
	return &MultiTierCache{
		l1:    NewMemoryCache(32, 1*time.Minute),
		redis: redis,
		l1TTL: l1TTL,
	}
}

// Get retrieves data from L1 (sub-millisecond) or falls back to L2 Redis.
func (c *MultiTierCache) Get(ctx context.Context, key string) (string, bool) {
	if c == nil {
		return "", false
	}
	// 1. Check L1 Memory Cache (nanosecond lookup)
	if c.l1 != nil {
		if val, ok := c.l1.Get(key); ok {
			return val, true
		}
	}

	// 2. Check L2 Redis Cache
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, key); err == nil && val != "" {
			// Populate L1 for subsequent requests
			if c.l1 != nil {
				c.l1.Set(key, val, c.l1TTL)
			}
			return val, true
		}
	}

	return "", false
}

// Set stores data in L2 Redis and L1 in-memory cache.
func (c *MultiTierCache) Set(ctx context.Context, key, value string, l2TTL time.Duration) {
	if c == nil {
		return
	}
	// Always set in L1
	if c.l1 != nil {
		c.l1.Set(key, value, c.l1TTL)
	}

	// Set in L2 Redis if available
	if c.redis != nil && l2TTL > 0 {
		_ = c.redis.Set(ctx, key, value, l2TTL)
	}
}

// Delete evicts a key from both L1 and L2 caches.
func (c *MultiTierCache) Delete(ctx context.Context, key string) {
	if c == nil {
		return
	}
	if c.l1 != nil {
		c.l1.Delete(key)
	}
	if c.redis != nil {
		_ = c.redis.Del(ctx, key)
	}
}

// Close releases background memory cache resources.
func (c *MultiTierCache) Close() {
	if c != nil && c.l1 != nil {
		c.l1.Close()
	}
}
