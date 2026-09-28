// Package finance calculates expected profitability for a property based
// on its cost and short-term-rental market data. All figures here are
// computed deterministically in Go so that the numbers handed to Jev and
// the LLM in later evaluation phases are exact, not model-estimated.
package finance

import (
	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

const daysPerMonth = 30.0

// MinpakuMaxDaysPerYear is the nationwide cap on annual operating days
// for a listing registered under 住宅宿泊事業法（民泊）. It does not
// apply to a full 旅館業許可 (property.BusinessTypeRyokan). Most
// individual short-term-rental operators use 民泊 registration, so this
// is the default assumption (property.Property's zero-value
// BusinessType).
const MinpakuMaxDaysPerYear = 180.0

// minpakuMaxOccupancyRate is MinpakuMaxDaysPerYear expressed as a
// fraction of the year, for comparison against market occupancy rates.
const minpakuMaxOccupancyRate = MinpakuMaxDaysPerYear / 365.0

// Result holds the computed profitability figures for a property.
//
// ROIPercent and NetYieldPercent use the same formula (annual profit
// over purchase price): this simplified model has no separate financing
// cost, so ROI and 実質利回り coincide. Both are kept as named fields so
// callers can refer to whichever term fits the context.
type Result struct {
	MonthlyRevenue    float64 // 想定月間収入（円）
	MonthlyProfit     float64 // 想定月間損益（円）
	AnnualProfit      float64 // 想定年間損益（円）
	ROIPercent        float64 // 年間ROI（購入価格に対する年間損益の割合、%）
	GrossYieldPercent float64 // 表面利回り（年間収入 ÷ 購入価格、%。費用を差し引かない）
	NetYieldPercent   float64 // 実質利回り（年間損益 ÷ 購入価格、%。ROIPercentと同値）
	BEPOccupancyRate  float64 // 損益分岐稼働率（月間損益がゼロになる稼働率、0.0-1.0）

	// EffectiveOccupancyRate is the occupancy rate actually used for
	// MonthlyRevenue: the market's OccupancyRate, clipped to
	// minpakuMaxOccupancyRate when the property is (or defaults to)
	// 民泊 registration.
	EffectiveOccupancyRate float64
	// LegalCapApplied is true when the market's occupancy rate exceeded
	// what 180 operating days/year allows, so real achievable revenue
	// is lower than market demand alone would suggest.
	LegalCapApplied bool
	// LegallyAchievable is false when BEPOccupancyRate itself exceeds
	// minpakuMaxOccupancyRate: the property cannot break even within
	// 180 operating days/year no matter how strong market demand is.
	// This is a hard legal ceiling, not a market risk.
	LegallyAchievable bool
}

// Calculate estimates monthly/annual profit, ROI, gross/net yield, and
// the break-even occupancy rate from a property's cost structure and the
// market data for its area, applying Japan's 180-day/year minpaku
// operating cap unless the property uses a full 旅館業許可.
func Calculate(p property.Property, m airdna.MarketData) Result {
	isMinpaku := p.BusinessType != property.BusinessTypeRyokan

	effectiveOccupancy := m.OccupancyRate
	legalCapApplied := false
	if isMinpaku && effectiveOccupancy > minpakuMaxOccupancyRate {
		effectiveOccupancy = minpakuMaxOccupancyRate
		legalCapApplied = true
	}

	monthlyRevenue := m.ADR * effectiveOccupancy * daysPerMonth
	monthlyProfit := monthlyRevenue - p.MonthlyRent
	annualProfit := monthlyProfit * 12
	annualRevenue := monthlyRevenue * 12

	var roi, grossYield, bepOccupancy float64
	if p.PurchasePrice != 0 {
		roi = annualProfit / p.PurchasePrice * 100
		grossYield = annualRevenue / p.PurchasePrice * 100
	}
	if m.ADR > 0 {
		bepOccupancy = p.MonthlyRent / (m.ADR * daysPerMonth)
	}

	legallyAchievable := !isMinpaku || bepOccupancy <= minpakuMaxOccupancyRate

	return Result{
		MonthlyRevenue:         monthlyRevenue,
		MonthlyProfit:          monthlyProfit,
		AnnualProfit:           annualProfit,
		ROIPercent:             roi,
		GrossYieldPercent:      grossYield,
		NetYieldPercent:        roi,
		BEPOccupancyRate:       bepOccupancy,
		EffectiveOccupancyRate: effectiveOccupancy,
		LegalCapApplied:        legalCapApplied,
		LegallyAchievable:      legallyAchievable,
	}
}
