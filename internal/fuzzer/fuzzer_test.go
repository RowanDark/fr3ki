package fuzzer

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRateLimiter_20JobsAt5RPS checks that 20 jobs at rate=5 take at least 4 seconds.
// At 5 req/s the first 5 consume the initial bucket; the remaining 15 each require
// waiting ~200ms, so total wall time is comfortably >= 4s.
func TestRateLimiter_20JobsAt5RPS(t *testing.T) {
	limiter := NewRateLimiter(5)
	jobs := make([]Job, 20)
	for i := range jobs {
		jobs[i] = func() {}
	}

	start := time.Now()
	// Use concurrency=20 so only the rate limiter is the bottleneck.
	WorkerPool(jobs, 20, limiter)
	elapsed := time.Since(start)

	if elapsed < 4*time.Second {
		t.Errorf("expected >= 4s with rate=5 and 20 jobs, got %v", elapsed)
	}
}

// TestWorkerPool_Concurrency checks that no more than limit jobs run simultaneously.
func TestWorkerPool_Concurrency(t *testing.T) {
	const limit = 3
	const numJobs = 15

	var (
		mu      sync.Mutex
		current int
		peak    int
	)

	limiter := NewRateLimiter(0) // unlimited rate so only concurrency is the bottleneck
	jobs := make([]Job, numJobs)
	for i := range jobs {
		jobs[i] = func() {
			mu.Lock()
			current++
			if current > peak {
				peak = current
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			current--
			mu.Unlock()
		}
	}

	WorkerPool(jobs, limit, limiter)

	if peak > limit {
		t.Errorf("peak concurrency %d exceeded limit %d", peak, limit)
	}
	if peak == 0 {
		t.Error("no jobs ran")
	}
}

// TestRateLimiter_NoOp verifies that rate=0 is a true no-op (no blocking).
func TestRateLimiter_NoOp(t *testing.T) {
	limiter := NewRateLimiter(0)
	start := time.Now()
	for i := 0; i < 100; i++ {
		limiter.Acquire()
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("rate=0 should be no-op but took %v", elapsed)
	}
}

// TestRateLimiter_TokenRefill checks that tokens refill over time.
func TestRateLimiter_TokenRefill(t *testing.T) {
	limiter := NewRateLimiter(10) // 10 rps = 1 token per 100ms

	// Drain initial bucket.
	for i := 0; i < 10; i++ {
		limiter.Acquire()
	}

	// Wait for 2 tokens to accumulate.
	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	limiter.Acquire()
	limiter.Acquire()
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Errorf("refilled tokens should be consumed quickly, got %v", elapsed)
	}
}

// TestWorkerPool_AllJobsRun verifies every job in the pool is executed exactly once.
func TestWorkerPool_AllJobsRun(t *testing.T) {
	const numJobs = 30
	var count atomic.Int64

	limiter := NewRateLimiter(0)
	jobs := make([]Job, numJobs)
	for i := range jobs {
		jobs[i] = func() { count.Add(1) }
	}

	WorkerPool(jobs, 5, limiter)

	if got := count.Load(); got != numJobs {
		t.Errorf("expected %d jobs to run, got %d", numJobs, got)
	}
}
