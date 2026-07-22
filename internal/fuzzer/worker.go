package fuzzer

import (
	"math/rand"
	"sync"
	"time"
)

// Job is a unit of work executed by the worker pool.
type Job func()

// WorkerPool runs jobs with bounded concurrency and per-job rate limiting + jitter.
// Each job acquires a token from limiter and sleeps a small random jitter before executing,
// matching the Python asyncio.Semaphore + RateLimiter.acquire() + asyncio.sleep pattern.
func WorkerPool(jobs []Job, concurrency int, limiter *RateLimiter) {
	if concurrency <= 0 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, job := range jobs {
		job := job
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			limiter.Acquire()
			jitter()
			job()
		}()
	}

	wg.Wait()
}

// jitter sleeps a random duration in [50ms, 300ms] to match Python's
// asyncio.sleep(random.uniform(0.05, 0.3)) per-request evasion delay.
func jitter() {
	d := 50 + rand.Intn(251) // 50..300 ms
	time.Sleep(time.Duration(d) * time.Millisecond)
}
