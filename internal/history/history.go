// Package history defines Revita's saved evaluation records — the
// input property, the market data and computed figures, Jev's verdict,
// and the report — so a user can look back at properties they've
// already evaluated and compare them against each other
// (internal/compare) without paying for a fresh Jev/LLM call.
package history

import (
	"time"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

// Record is one saved evaluation run.
type Record struct {
	ID         string
	UserID     string
	Property   property.Property
	Market     airdna.MarketData
	Finance    finance.Result
	Evaluation jev.Evaluation
	Report     string
	CreatedAt  time.Time
}

// Store persists and retrieves evaluation history, scoped per user.
// The Postgres-backed implementation lives in internal/db; tests use
// an in-memory fake.
type Store interface {
	// Save persists rec and returns its assigned ID.
	Save(rec Record) (id string, err error)
	// List returns userID's most recent records, newest first, up to
	// limit.
	List(userID string, limit int) ([]Record, error)
	// Get returns userID's records matching ids, in no particular
	// order. IDs that don't exist, or belong to a different user, are
	// silently omitted rather than erroring — callers should check the
	// returned count against len(ids) if they need to detect that.
	Get(userID string, ids []string) ([]Record, error)
}
