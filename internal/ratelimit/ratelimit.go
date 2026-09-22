// Package ratelimit enforces a fixed monthly execution limit per user,
// so a single ¥300/month subscriber can't run unbounded Jev/LLM API
// calls on Revita's dime.
package ratelimit

import "time"

// UsageStore persists monthly usage counts. The Postgres-backed
// implementation lives in internal/db; tests use an in-memory fake.
type UsageStore interface {
	// IncrementAndGet atomically increments the counter for
	// (userID, yearMonth) and returns the new count.
	IncrementAndGet(userID, yearMonth string) (int, error)
}

// Limiter enforces Limit executions per calendar month per user.
type Limiter struct {
	Store UsageStore
	Limit int
	Now   func() time.Time // overridable in tests
}

// NewLimiter returns a Limiter backed by store, allowing up to limit
// executions per user per calendar month (UTC).
func NewLimiter(store UsageStore, limit int) *Limiter {
	return &Limiter{Store: store, Limit: limit, Now: time.Now}
}

// Allow increments the current month's usage counter for userID and
// reports whether the request is within the monthly limit. Once a user
// exceeds the limit, every subsequent call this month keeps returning
// allowed=false (the counter keeps counting past the limit, which is
// fine since only its relation to Limit matters).
func (l *Limiter) Allow(userID string) (allowed bool, remaining int, err error) {
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	yearMonth := now().UTC().Format("2006-01")

	count, err := l.Store.IncrementAndGet(userID, yearMonth)
	if err != nil {
		return false, 0, err
	}
	if count > l.Limit {
		return false, 0, nil
	}
	return true, l.Limit - count, nil
}
