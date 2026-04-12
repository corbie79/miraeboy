package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// tokenBucket implements the token bucket algorithm for a single IP.
type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

func newTokenBucket(rps float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: rps,
		lastRefill: time.Now(),
	}
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.maxTokens, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// rateLimiter manages per-IP token buckets with periodic cleanup.
type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rps     float64
	burst   int
}

func newRateLimiter(rps float64, burst int) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rps:     rps,
		burst:   burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	b, ok := rl.buckets[ip]
	if !ok {
		b = newTokenBucket(rl.rps, rl.burst)
		rl.buckets[ip] = b
	}
	rl.mu.Unlock()
	return b.allow()
}

// cleanup removes buckets for IPs that haven't been seen in 10 minutes.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			b.mu.Lock()
			if b.lastRefill.Before(cutoff) {
				delete(rl.buckets, ip)
			}
			b.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// rateLimitMiddleware wraps a handler and enforces per-IP rate limiting.
// Returns 429 when the bucket is empty. Skipped when rl is nil.
func rateLimitMiddleware(rl *rateLimiter, next http.Handler) http.Handler {
	if rl == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		// Trust X-Forwarded-For from localhost (reverse proxy)
		if ip == "127.0.0.1" || ip == "::1" {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				ip = xff
			}
		}
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "1")
			jsonError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}
