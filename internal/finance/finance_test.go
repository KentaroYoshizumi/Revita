package finance

import (
	"math"
	"testing"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}

func TestCalculate(t *testing.T) {
	p := property.Property{PurchasePrice: 45000000, MonthlyRent: 150000}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.68}

	r := Calculate(p, m)

	wantMonthlyRevenue := 18500.0 * 0.68 * 30.0
	if !almostEqual(r.MonthlyRevenue, wantMonthlyRevenue) {
		t.Errorf("MonthlyRevenue = %v, want %v", r.MonthlyRevenue, wantMonthlyRevenue)
	}

	wantMonthlyProfit := wantMonthlyRevenue - 150000
	if !almostEqual(r.MonthlyProfit, wantMonthlyProfit) {
		t.Errorf("MonthlyProfit = %v, want %v", r.MonthlyProfit, wantMonthlyProfit)
	}

	wantAnnualProfit := wantMonthlyProfit * 12
	if !almostEqual(r.AnnualProfit, wantAnnualProfit) {
		t.Errorf("AnnualProfit = %v, want %v", r.AnnualProfit, wantAnnualProfit)
	}

	wantROI := wantAnnualProfit / 45000000 * 100
	if !almostEqual(r.ROIPercent, wantROI) {
		t.Errorf("ROIPercent = %v, want %v", r.ROIPercent, wantROI)
	}
	if !almostEqual(r.NetYieldPercent, wantROI) {
		t.Errorf("NetYieldPercent = %v, want %v (same formula as ROI)", r.NetYieldPercent, wantROI)
	}

	wantGrossYield := (wantMonthlyRevenue * 12) / 45000000 * 100
	if !almostEqual(r.GrossYieldPercent, wantGrossYield) {
		t.Errorf("GrossYieldPercent = %v, want %v", r.GrossYieldPercent, wantGrossYield)
	}

	wantBEP := 150000.0 / (18500.0 * 30.0)
	if !almostEqual(r.BEPOccupancyRate, wantBEP) {
		t.Errorf("BEPOccupancyRate = %v, want %v", r.BEPOccupancyRate, wantBEP)
	}
}

func TestCalculate_ZeroPurchasePriceAndADR(t *testing.T) {
	p := property.Property{PurchasePrice: 0, MonthlyRent: 100000}
	m := airdna.MarketData{ADR: 0, OccupancyRate: 0.5}

	r := Calculate(p, m)

	if r.ROIPercent != 0 || r.GrossYieldPercent != 0 {
		t.Errorf("expected ROIPercent/GrossYieldPercent to be 0 when PurchasePrice is 0, got %+v", r)
	}
	if r.BEPOccupancyRate != 0 {
		t.Errorf("expected BEPOccupancyRate to be 0 when ADR is 0, got %v", r.BEPOccupancyRate)
	}
}
