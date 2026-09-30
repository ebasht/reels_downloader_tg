package usecase

import (
	"sync"
	"time"
)

// RateLimiter allows at most limit events per key within a fixed window.
type RateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu        sync.Mutex
	buckets   map[string]*bucket
	lastSweep time.Time
}

type bucket struct {
	start    time.Time
	count    int
	rejected bool
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		limit:   limit,
		window:  window,
		now:     time.Now,
		buckets: make(map[string]*bucket),
	}
}

// Allow records an event for key. firstRejection is true only for the first
// rejected event in a window, so the caller can warn the user once.
func (l *RateLimiter) Allow(key string) (allowed, firstRejection bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.start) >= l.window {
		b = &bucket{start: now}
		l.buckets[key] = b
	}
	if b.count < l.limit {
		b.count++
		return true, false
	}
	first := !b.rejected
	b.rejected = true
	return false, first
}

func (l *RateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	for key, b := range l.buckets {
		if now.Sub(b.start) >= l.window {
			delete(l.buckets, key)
		}
	}
	l.lastSweep = now
}
