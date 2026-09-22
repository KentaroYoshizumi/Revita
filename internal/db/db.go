// Package db is the Postgres-backed persistence layer for Revita's
// SaaS state (subscriptions, monthly usage counters), intended to run
// against a Supabase project's Postgres database. It implements the
// storage interfaces expected by internal/billing and
// internal/ratelimit.
//
// This package is not covered by this repository's automated tests: no
// Postgres instance is available in the development sandbox that wrote
// it. internal/billing and internal/ratelimit are tested against fakes
// instead; verify this package against a real Supabase database before
// relying on it in production.
package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/KentaroYoshizumi/Revita/internal/billing"
	"github.com/KentaroYoshizumi/Revita/internal/ratelimit"
	_ "github.com/lib/pq"
)

// Compile-time checks that *DB satisfies the storage interfaces
// internal/billing and internal/ratelimit depend on.
var (
	_ billing.SubscriptionStore = (*DB)(nil)
	_ ratelimit.UsageStore      = (*DB)(nil)
)

// DB wraps a Postgres connection pool.
type DB struct {
	*sql.DB
}

// Open connects to Postgres at dsn (e.g. the Supabase project's
// connection string) and verifies the connection with a ping.
func Open(dsn string) (*DB, error) {
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: failed to open connection: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("db: ping failed: %w", err)
	}
	return &DB{sqlDB}, nil
}

// IncrementAndGet implements ratelimit.UsageStore: it atomically
// increments the (userID, yearMonth) counter in public.usage_counters
// and returns the new count.
func (d *DB) IncrementAndGet(userID, yearMonth string) (int, error) {
	var count int
	err := d.QueryRow(`
		INSERT INTO public.usage_counters (user_id, year_month, count, updated_at)
		VALUES ($1, $2, 1, now())
		ON CONFLICT (user_id, year_month)
		DO UPDATE SET count = public.usage_counters.count + 1, updated_at = now()
		RETURNING count
	`, userID, yearMonth).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("db: increment usage failed: %w", err)
	}
	return count, nil
}

// Upsert implements billing.SubscriptionStore: it creates or replaces
// the subscription row for rec.UserID.
func (d *DB) Upsert(rec billing.SubscriptionRecord) error {
	_, err := d.Exec(`
		INSERT INTO public.subscriptions
			(user_id, stripe_customer_id, stripe_subscription_id, status, current_period_end, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (user_id) DO UPDATE SET
			stripe_customer_id = excluded.stripe_customer_id,
			stripe_subscription_id = excluded.stripe_subscription_id,
			status = excluded.status,
			current_period_end = excluded.current_period_end,
			updated_at = now()
	`, rec.UserID, rec.StripeCustomerID, rec.StripeSubscriptionID, rec.Status, nullableTime(rec.CurrentPeriodEnd))
	if err != nil {
		return fmt.Errorf("db: upsert subscription failed: %w", err)
	}
	return nil
}

// UpdateStatus implements billing.SubscriptionStore: it updates the
// status/period-end of the subscription row identified by
// stripeSubscriptionID.
func (d *DB) UpdateStatus(stripeSubscriptionID, status string, currentPeriodEnd time.Time) error {
	_, err := d.Exec(`
		UPDATE public.subscriptions
		SET status = $2, current_period_end = $3, updated_at = now()
		WHERE stripe_subscription_id = $1
	`, stripeSubscriptionID, status, nullableTime(currentPeriodEnd))
	if err != nil {
		return fmt.Errorf("db: update subscription status failed: %w", err)
	}
	return nil
}

// GetSubscriptionStatus returns the current subscription status for
// userID, or "none" if no subscription row exists yet.
func (d *DB) GetSubscriptionStatus(userID string) (string, error) {
	var status string
	err := d.QueryRow(`SELECT status FROM public.subscriptions WHERE user_id = $1`, userID).Scan(&status)
	if err == sql.ErrNoRows {
		return "none", nil
	}
	if err != nil {
		return "", fmt.Errorf("db: get subscription status failed: %w", err)
	}
	return status, nil
}

func nullableTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
