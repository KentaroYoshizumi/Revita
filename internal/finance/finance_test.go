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

// TestCalculate uses an occupancy rate below the minpaku 180-day cap
// (180/365 ≈ 49.3%) so the legal cap doesn't interfere with the basic
// revenue/ROI/yield/BEP arithmetic being verified here.
func TestCalculate(t *testing.T) {
	p := property.Property{PurchasePrice: 45000000, MonthlyRent: 150000}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.4}

	r := Calculate(p, m)

	wantMonthlyRevenue := 18500.0 * 0.4 * 30.0
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

	if r.LegalCapApplied {
		t.Error("LegalCapApplied should be false when market occupancy is below the 180-day cap")
	}
	if !r.LegallyAchievable {
		t.Error("LegallyAchievable should be true for this low-cost scenario")
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

func TestCalculate_MinpakuCapLimitsRevenue(t *testing.T) {
	p := property.Property{PurchasePrice: 45000000, MonthlyRent: 150000} // BusinessType zero-value == minpaku
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.68}              // above the ~49.3% cap

	r := Calculate(p, m)

	wantEffectiveOccupancy := MinpakuMaxDaysPerYear / 365.0
	if !almostEqual(r.EffectiveOccupancyRate, wantEffectiveOccupancy) {
		t.Errorf("EffectiveOccupancyRate = %v, want %v (clipped to the 180-day cap)", r.EffectiveOccupancyRate, wantEffectiveOccupancy)
	}
	if !r.LegalCapApplied {
		t.Error("LegalCapApplied should be true when market occupancy exceeds the 180-day cap")
	}

	wantMonthlyRevenue := 18500.0 * wantEffectiveOccupancy * 30.0
	if !almostEqual(r.MonthlyRevenue, wantMonthlyRevenue) {
		t.Errorf("MonthlyRevenue = %v, want %v (based on capped occupancy, not raw market occupancy)", r.MonthlyRevenue, wantMonthlyRevenue)
	}
}

func TestCalculate_RyokanIgnoresCap(t *testing.T) {
	p := property.Property{PurchasePrice: 45000000, MonthlyRent: 150000, BusinessType: property.BusinessTypeRyokan}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.68}

	r := Calculate(p, m)

	if r.LegalCapApplied {
		t.Error("LegalCapApplied should be false for a 旅館業許可 property")
	}
	if !almostEqual(r.EffectiveOccupancyRate, 0.68) {
		t.Errorf("EffectiveOccupancyRate = %v, want 0.68 (uncapped)", r.EffectiveOccupancyRate)
	}
}

func TestCalculate_LegallyUnachievable(t *testing.T) {
	// A very high monthly rent relative to ADR pushes the break-even
	// occupancy rate above what 180 days/year of minpaku operation can
	// ever reach, regardless of market demand.
	p := property.Property{PurchasePrice: 45000000, MonthlyRent: 400000}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.9}

	r := Calculate(p, m)

	if r.LegallyAchievable {
		t.Errorf("expected LegallyAchievable = false when BEPOccupancyRate (%v) exceeds the 180-day cap", r.BEPOccupancyRate)
	}
}
