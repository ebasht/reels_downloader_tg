package usecase

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewRateLimiter(2, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("u"); !ok {
			t.Fatalf("event %d rejected", i)
		}
	}
	if ok, first := l.Allow("u"); ok || !first {
		t.Fatalf("3rd event: ok=%v first=%v, want false/true", ok, first)
	}
	if ok, first := l.Allow("u"); ok || first {
		t.Fatalf("4th event: ok=%v first=%v, want false/false", ok, first)
	}
	if ok, _ := l.Allow("other"); !ok {
		t.Fatal("other key rejected")
	}

	now = now.Add(time.Minute)
	if ok, _ := l.Allow("u"); !ok {
		t.Fatal("event after window rejected")
	}
}
