package auth

import (
	"testing"
	"time"
)

func TestRateLimiterBurstAndRefill(t *testing.T) {
	l := NewRateLimiter(2, time.Minute)
	now := time.Now()
	for i := range 2 {
		if ok, _ := l.Allow("ip", now); !ok {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
	if ok, wait := l.Allow("ip", now); ok || wait <= 0 {
		t.Fatalf("third attempt should be denied with a positive wait, got ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allow("ip", now.Add(2*time.Minute)); !ok {
		t.Fatal("bucket should have refilled after two refill intervals")
	}
}

func TestRateLimiterCapsBuckets(t *testing.T) {
	l := NewRateLimiter(1, time.Hour)
	l.MaxBuckets = 2
	now := time.Now()
	l.Allow("a", now)
	l.Allow("b", now)
	// Beyond the cap, unknown keys collapse into one shared overflow bucket instead of growing the map.
	if ok, _ := l.Allow("c", now); !ok {
		t.Fatal("first overflow key should get the overflow bucket's burst")
	}
	if ok, _ := l.Allow("d", now); ok {
		t.Fatal("further unknown keys must share the exhausted overflow bucket")
	}
	l.mu.Lock()
	n := len(l.buckets)
	l.mu.Unlock()
	if n != 3 {
		t.Fatalf("map should hold a, b and the overflow bucket only, got %d", n)
	}
	// Known keys still work normally past the cap.
	if ok, _ := l.Allow("a", now.Add(2*time.Hour)); !ok {
		t.Fatal("known key should refill independently of the overflow bucket")
	}
}
