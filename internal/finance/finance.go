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
}

// Calculate estimates monthly/annual profit, ROI, gross/net yield, and
// the break-even occupancy rate from a property's cost structure and the
// market data for its area.
func Calculate(p property.Property, m airdna.MarketData) Result {
	monthlyRevenue := m.ADR * m.OccupancyRate * daysPerMonth
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

	return Result{
		MonthlyRevenue:    monthlyRevenue,
		MonthlyProfit:     monthlyProfit,
		AnnualProfit:      annualProfit,
		ROIPercent:        roi,
		GrossYieldPercent: grossYield,
		NetYieldPercent:   roi,
		BEPOccupancyRate:  bepOccupancy,
	}
}
