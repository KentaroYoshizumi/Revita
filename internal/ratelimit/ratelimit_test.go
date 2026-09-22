package ratelimit

import (
	"sync"
	"testing"
	"time"
)

// fakeStore is an in-memory UsageStore for tests, since no real
// Postgres instance is available in this environment.
type fakeStore struct {
	mu     sync.Mutex
	counts map[string]int // key: userID+"|"+yearMonth
}

func newFakeStore() *fakeStore {
	return &fakeStore{counts: map[string]int{}}
}

func (s *fakeStore) IncrementAndGet(userID, yearMonth string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + "|" + yearMonth
	s.counts[key]++
	return s.counts[key], nil
}

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestLimiter_AllowsWithinLimit(t *testing.T) {
	store := newFakeStore()
	limiter := NewLimiter(store, 3)
	limiter.Now = fixedNow(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))

	for i := 1; i <= 3; i++ {
		allowed, remaining, err := limiter.Allow("user-1")
		if err != nil {
			t.Fatalf("Allow returned error: %v", err)
		}
		if !allowed {
			t.Errorf("call %d: allowed = false, want true", i)
		}
		if remaining != 3-i {
			t.Errorf("call %d: remaining = %d, want %d", i, remaining, 3-i)
		}
	}
}

func TestLimiter_BlocksOverLimit(t *testing.T) {
	store := newFakeStore()
	limiter := NewLimiter(store, 2)
	limiter.Now = fixedNow(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))

	for i := 0; i < 2; i++ {
		if allowed, _, _ := limiter.Allow("user-1"); !allowed {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}

	allowed, remaining, err := limiter.Allow("user-1")
	if err != nil {
		t.Fatalf("Allow returned error: %v", err)
	}
	if allowed {
		t.Error("3rd call should be blocked (limit is 2)")
	}
	if remaining != 0 {
		t.Errorf("remaining = %d, want 0", remaining)
	}
}

func TestLimiter_ResetsNextMonth(t *testing.T) {
	store := newFakeStore()
	limiter := NewLimiter(store, 1)

	limiter.Now = fixedNow(time.Date(2026, 9, 30, 23, 0, 0, 0, time.UTC))
	if allowed, _, _ := limiter.Allow("user-1"); !allowed {
		t.Fatal("first call in September should be allowed")
	}
	if allowed, _, _ := limiter.Allow("user-1"); allowed {
		t.Fatal("second call in September should be blocked")
	}

	limiter.Now = fixedNow(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if allowed, _, _ := limiter.Allow("user-1"); !allowed {
		t.Fatal("first call in October should be allowed (counter reset)")
	}
}

func TestLimiter_PerUserIsolation(t *testing.T) {
	store := newFakeStore()
	limiter := NewLimiter(store, 1)
	limiter.Now = fixedNow(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))

	if allowed, _, _ := limiter.Allow("user-1"); !allowed {
		t.Fatal("user-1's first call should be allowed")
	}
	if allowed, _, _ := limiter.Allow("user-2"); !allowed {
		t.Fatal("user-2's first call should be allowed independently of user-1")
	}
}
