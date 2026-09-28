package compare

import (
	"testing"

	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/history"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

func rec(id, address string, netYield, effectiveOcc, bep float64, legallyAchievable bool) history.Record {
	return history.Record{
		ID:       id,
		Property: property.Property{Address: address},
		Finance: finance.Result{
			NetYieldPercent:        netYield,
			EffectiveOccupancyRate: effectiveOcc,
			BEPOccupancyRate:       bep,
			LegallyAchievable:      legallyAchievable,
		},
	}
}

func TestCompare_RanksByNetYieldDescending(t *testing.T) {
	records := []history.Record{
		rec("a", "物件A", 5, 0.5, 0.3, true),
		rec("b", "物件B", 12, 0.5, 0.3, true),
		rec("c", "物件C", 8, 0.5, 0.3, true),
	}

	result := Compare(records)

	if len(result.ByNetYield) != 3 || result.ByNetYield[0].ID != "b" || result.ByNetYield[1].ID != "c" || result.ByNetYield[2].ID != "a" {
		t.Errorf("ByNetYield order = %+v, want b,c,a", result.ByNetYield)
	}
}

func TestCompare_RanksByBEPMarginDescending(t *testing.T) {
	records := []history.Record{
		rec("a", "物件A", 5, 0.5, 0.45, true), // margin 5pt
		rec("b", "物件B", 5, 0.6, 0.2, true),  // margin 40pt
	}

	result := Compare(records)

	if result.ByBEPMargin[0].ID != "b" {
		t.Errorf("ByBEPMargin[0].ID = %q, want %q (larger margin)", result.ByBEPMargin[0].ID, "b")
	}
}

func TestCompare_RecommendsBestLegallyAchievable(t *testing.T) {
	records := []history.Record{
		rec("a", "物件A", 20, 0.5, 0.3, false), // best yield but legally unachievable
		rec("b", "物件B", 8, 0.5, 0.3, true),
	}

	result := Compare(records)

	if result.RecommendedID != "b" {
		t.Errorf("RecommendedID = %q, want %q (highest yield among legally achievable candidates)", result.RecommendedID, "b")
	}
}

func TestCompare_NoRecommendationWhenNoneAchievable(t *testing.T) {
	records := []history.Record{
		rec("a", "物件A", 20, 0.5, 0.3, false),
		rec("b", "物件B", 8, 0.5, 0.3, false),
	}

	result := Compare(records)

	if result.RecommendedID != "" {
		t.Errorf("RecommendedID = %q, want empty when no candidate is legally achievable", result.RecommendedID)
	}
}

func TestCompare_Empty(t *testing.T) {
	result := Compare(nil)
	if result.Narrative == "" {
		t.Error("expected a non-empty narrative even for an empty comparison")
	}
}
