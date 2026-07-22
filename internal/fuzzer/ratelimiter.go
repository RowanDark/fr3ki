package fuzzer

import (
	"sync"
	"time"
)

// RateLimiter is a token-bucket rate limiter.
// Tokens refill continuously based on elapsed wall-clock time, capped at rate.
// The bucket starts empty so every caller must earn a token.
// Callers block (mutex held) until a token is available.
// A rate of 0 disables all limiting.
type RateLimiter struct {
	rate      float64
	tokens    float64
	lastRefil time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a RateLimiter with the given requests-per-second rate.
func NewRateLimiter(rate float64) *RateLimiter {
	return &RateLimiter{
		rate:      rate,
		tokens:    0,
		lastRefil: time.Now(),
	}
}

// Acquire blocks until a token is available, then consumes one token.
// It is a no-op when rate <= 0.
func (r *RateLimiter) Acquire() {
	if r.rate <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefil).Seconds()
	r.tokens += elapsed * r.rate
	if r.tokens > r.rate {
		r.tokens = r.rate
	}

	if r.tokens < 1 {
		wait := time.Duration((1-r.tokens)/r.rate*1e9) * time.Nanosecond
		time.Sleep(wait)
		r.tokens = 0
		// Update lastRefil AFTER the sleep so the next caller does not inherit
		// the elapsed time from this sleep as free tokens.
		r.lastRefil = time.Now()
	} else {
		r.tokens -= 1
		r.lastRefil = now
	}
}
