package database

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMemoryCache_BasicOperations(t *testing.T) {
	cache := NewMemoryCache(8, 100*time.Millisecond)
	defer cache.Close()

	// Get non-existent
	if _, found := cache.Get("key1"); found {
		t.Fatal("expected key1 to not exist")
	}

	// Set and Get
	cache.Set("key1", "val1", 1*time.Minute)
	val, found := cache.Get("key1")
	if !found || val != "val1" {
		t.Fatalf("expected val1, got %q (found: %v)", val, found)
	}

	// Delete
	cache.Delete("key1")
	if _, found := cache.Get("key1"); found {
		t.Fatal("expected key1 to be deleted")
	}
}

func TestMemoryCache_TTLExpiration(t *testing.T) {
	cache := NewMemoryCache(4, 50*time.Millisecond)
	defer cache.Close()

	cache.Set("short_key", "short_val", 20*time.Millisecond)

	val, found := cache.Get("short_key")
	if !found || val != "short_val" {
		t.Fatalf("expected short_val before expiry, got %q", val)
	}

	time.Sleep(35 * time.Millisecond)

	if _, found := cache.Get("short_key"); found {
		t.Fatal("expected short_key to be expired after TTL")
	}
}

func TestMemoryCache_Concurrency(t *testing.T) {
	cache := NewMemoryCache(16, 1*time.Second)
	defer cache.Close()

	var wg sync.WaitGroup
	workers := 50
	opsPerWorker := 200

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				key := fmt.Sprintf("k_%d_%d", workerID, j)
				val := fmt.Sprintf("v_%d_%d", workerID, j)
				cache.Set(key, val, 1*time.Minute)

				got, found := cache.Get(key)
				if !found || got != val {
					t.Errorf("worker %d: expected %s, got %s", workerID, val, got)
				}

				if j%5 == 0 {
					cache.Delete(key)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestMultiTierCache_MemoryFallback(t *testing.T) {
	ctx := context.Background()
	mtc := NewMultiTierCache(nil, 50*time.Millisecond)
	defer mtc.Close()

	// Get non-existent
	if _, ok := mtc.Get(ctx, "mtc_key"); ok {
		t.Fatal("expected mtc_key to not exist")
	}

	// Set and Get
	mtc.Set(ctx, "mtc_key", "mtc_val", 1*time.Minute)
	val, ok := mtc.Get(ctx, "mtc_key")
	if !ok || val != "mtc_val" {
		t.Fatalf("expected mtc_val, got %s", val)
	}

	// Delete
	mtc.Delete(ctx, "mtc_key")
	if _, ok := mtc.Get(ctx, "mtc_key"); ok {
		t.Fatal("expected mtc_key to be deleted")
	}

	// Idempotent Close
	mtc.Close()
	mtc.Close()
}

func BenchmarkMemoryCache_Get(b *testing.B) {
	cache := NewMemoryCache(32, 1*time.Minute)
	defer cache.Close()

	cache.Set("bench_key", "bench_value", 5*time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			val, found := cache.Get("bench_key")
			if !found || val != "bench_value" {
				b.Fatalf("cache lookup failed")
			}
		}
	})
}

func BenchmarkMemoryCache_Set(b *testing.B) {
	cache := NewMemoryCache(32, 1*time.Minute)
	defer cache.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			key := fmt.Sprintf("k_%d", i%1000)
			cache.Set(key, "bench_val", 5*time.Minute)
		}
	})
}
