package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"xsocial-backend/internal/utils"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*ipLimiter
	rate     rate.Limit
	burst    int
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*ipLimiter),
		rate:     r,
		burst:    b,
	}
	// Cleanup stale entries every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			now := time.Now()
			for ip, item := range rl.limiters {
				if now.Sub(item.lastSeen) > 10*time.Minute {
					delete(rl.limiters, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		rl.mu.Lock()
		lim, exists := rl.limiters[ip]
		if !exists {
			lim = &ipLimiter{
				limiter:  rate.NewLimiter(rl.rate, rl.burst),
				lastSeen: time.Now(),
			}
			rl.limiters[ip] = lim
		}
		lim.lastSeen = time.Now()
		allowed := lim.limiter.Allow()
		rl.mu.Unlock()

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, utils.APIResponse{
				Success: false,
				Message: "Rate limit exceeded. Please try again later.",
			})
			return
		}

		c.Next()
	}
}
