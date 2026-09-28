package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/history"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/property"
	"github.com/lib/pq"
)

// Compile-time check that *DB satisfies history.Store.
var _ history.Store = (*DB)(nil)

// Save implements history.Store: it inserts rec into public.evaluations
// and returns the generated row ID.
func (d *DB) Save(rec history.Record) (string, error) {
	marketJSON, err := json.Marshal(rec.Market)
	if err != nil {
		return "", fmt.Errorf("db: failed to marshal market data: %w", err)
	}
	financeJSON, err := json.Marshal(rec.Finance)
	if err != nil {
		return "", fmt.Errorf("db: failed to marshal finance result: %w", err)
	}
	evaluationJSON, err := json.Marshal(rec.Evaluation)
	if err != nil {
		return "", fmt.Errorf("db: failed to marshal evaluation: %w", err)
	}

	businessType := string(rec.Property.BusinessType)
	if businessType == "" {
		businessType = string(property.BusinessTypeMinpaku)
	}

	var id string
	err = d.QueryRow(`
		INSERT INTO public.evaluations
			(user_id, address, purchase_price, monthly_rent, size_sqm, capacity, business_type,
			 market_data, finance_result, evaluation, report_markdown)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`,
		rec.UserID, rec.Property.Address, rec.Property.PurchasePrice, rec.Property.MonthlyRent,
		rec.Property.SizeSqm, rec.Property.Capacity, businessType,
		marketJSON, financeJSON, evaluationJSON, rec.Report,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("db: failed to save evaluation: %w", err)
	}
	return id, nil
}

// List implements history.Store: it returns userID's most recent
// evaluations, newest first.
func (d *DB) List(userID string, limit int) ([]history.Record, error) {
	rows, err := d.Query(`
		SELECT id, user_id, address, purchase_price, monthly_rent, size_sqm, capacity, business_type,
		       market_data, finance_result, evaluation, report_markdown, created_at
		FROM public.evaluations
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: failed to list evaluations: %w", err)
	}
	defer rows.Close()

	return scanEvaluationRows(rows)
}

// Get implements history.Store: it returns userID's evaluations among
// ids, silently omitting any id that doesn't exist or belongs to a
// different user.
func (d *DB) Get(userID string, ids []string) ([]history.Record, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := d.Query(`
		SELECT id, user_id, address, purchase_price, monthly_rent, size_sqm, capacity, business_type,
		       market_data, finance_result, evaluation, report_markdown, created_at
		FROM public.evaluations
		WHERE user_id = $1 AND id = ANY($2)
	`, userID, pq.Array(ids))
	if err != nil {
		return nil, fmt.Errorf("db: failed to get evaluations: %w", err)
	}
	defer rows.Close()

	return scanEvaluationRows(rows)
}

func scanEvaluationRows(rows *sql.Rows) ([]history.Record, error) {
	var records []history.Record
	for rows.Next() {
		var (
			rec                                     history.Record
			businessType                            string
			marketJSON, financeJSON, evaluationJSON []byte
		)
		if err := rows.Scan(
			&rec.ID, &rec.UserID, &rec.Property.Address, &rec.Property.PurchasePrice, &rec.Property.MonthlyRent,
			&rec.Property.SizeSqm, &rec.Property.Capacity, &businessType,
			&marketJSON, &financeJSON, &evaluationJSON, &rec.Report, &rec.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("db: failed to scan evaluation row: %w", err)
		}
		rec.Property.BusinessType = property.BusinessType(businessType)

		var market airdna.MarketData
		if err := json.Unmarshal(marketJSON, &market); err != nil {
			return nil, fmt.Errorf("db: failed to unmarshal market data for evaluation %s: %w", rec.ID, err)
		}
		rec.Market = market

		var fin finance.Result
		if err := json.Unmarshal(financeJSON, &fin); err != nil {
			return nil, fmt.Errorf("db: failed to unmarshal finance result for evaluation %s: %w", rec.ID, err)
		}
		rec.Finance = fin

		var eval jev.Evaluation
		if err := json.Unmarshal(evaluationJSON, &eval); err != nil {
			return nil, fmt.Errorf("db: failed to unmarshal evaluation for evaluation %s: %w", rec.ID, err)
		}
		rec.Evaluation = eval

		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("db: error iterating evaluation rows: %w", err)
	}
	return records, nil
}
