package auth

import (
	"sync"
	"time"
)

// maxBuckets caps the map so a flood of spoofed client keys can't exhaust memory. Past the cap,
// unknown keys share one overflow bucket (fail closed) instead of growing the map further.
const maxBuckets = 100_000

// overflowKey is the shared bucket for keys beyond maxBuckets.
const overflowKey = "\x00overflow"

// RateLimiter is an in-memory token bucket per key (client IP). Burst attempts are allowed at once,
// then one more every Refill.
type RateLimiter struct {
	Burst  float64
	Refill time.Duration
	// MaxBuckets caps the bucket map; zero means the default maxBuckets.
	MaxBuckets int

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastPrune time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewRateLimiter(burst int, refill time.Duration) *RateLimiter {
	return &RateLimiter{Burst: float64(burst), Refill: refill, MaxBuckets: maxBuckets, buckets: map[string]*bucket{}}
}

// Allow takes a token for key, reporting false (and how long until the next token) when the bucket is empty.
func (l *RateLimiter) Allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune(now)
	max := l.MaxBuckets
	if max <= 0 {
		max = maxBuckets
	}
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= max {
			key = overflowKey
			b, ok = l.buckets[key]
		}
		if !ok {
			b = &bucket{tokens: l.Burst, last: now}
			l.buckets[key] = b
		}
	}
	b.tokens = min(l.Burst, b.tokens+float64(now.Sub(b.last))/float64(l.Refill))
	b.last = now
	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) * float64(l.Refill))
	}
	b.tokens--
	return true, 0
}

// prune drops buckets that have refilled completely, at most every ten minutes.
func (l *RateLimiter) prune(now time.Time) {
	if now.Sub(l.lastPrune) < 10*time.Minute {
		return
	}
	l.lastPrune = now
	full := time.Duration(l.Burst * float64(l.Refill))
	for k, b := range l.buckets {
		if now.Sub(b.last) > full {
			delete(l.buckets, k)
		}
	}
}
