// Package finance calculates expected profitability for a property based
// on its cost and short-term-rental market data.
package finance

import (
	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

const daysPerMonth = 30.0

// Result holds the computed profitability figures for a property.
type Result struct {
	MonthlyRevenue float64 // 想定月間収入（円）
	MonthlyProfit  float64 // 想定月間損益（円）
	AnnualProfit   float64 // 想定年間損益（円）
	ROIPercent     float64 // 年間ROI（購入価格に対する年間損益の割合、%）
}

// Calculate estimates monthly/annual profit and ROI from a property's
// cost structure and the market data for its area.
func Calculate(p property.Property, m airdna.MarketData) Result {
	monthlyRevenue := m.ADR * m.OccupancyRate * daysPerMonth
	monthlyProfit := monthlyRevenue - p.MonthlyRent
	annualProfit := monthlyProfit * 12

	var roi float64
	if p.PurchasePrice != 0 {
		roi = annualProfit / p.PurchasePrice * 100
	}

	return Result{
		MonthlyRevenue: monthlyRevenue,
		MonthlyProfit:  monthlyProfit,
		AnnualProfit:   annualProfit,
		ROIPercent:     roi,
	}
}
