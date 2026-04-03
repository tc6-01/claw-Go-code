package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateLimiter struct {
	mu       sync.Mutex
	counters map[string]*tokenBucket
	rate     int
	window   time.Duration
}

type tokenBucket struct {
	tokens    int
	lastReset time.Time
}

func newRateLimiter(requestsPerMinute int) *rateLimiter {
	return &rateLimiter{
		counters: make(map[string]*tokenBucket),
		rate:     requestsPerMinute,
		window:   time.Minute,
	}
}

func (rl *rateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, ok := rl.counters[key]
	if !ok {
		rl.counters[key] = &tokenBucket{tokens: rl.rate - 1, lastReset: now}
		return true
	}

	if now.Sub(bucket.lastReset) >= rl.window {
		bucket.tokens = rl.rate - 1
		bucket.lastReset = now
		return true
	}

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}
	return false
}

func rateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	if requestsPerMinute <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	limiter := newRateLimiter(requestsPerMinute)

	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		key := c.GetString(string(ctxKeyIdentity))
		if key == "" {
			key = c.ClientIP()
		}

		if !limiter.Allow(key) {
			writeError(c, http.StatusTooManyRequests, ErrRateLimit, "rate limit exceeded")
			c.Abort()
			return
		}
		c.Next()
	}
}
