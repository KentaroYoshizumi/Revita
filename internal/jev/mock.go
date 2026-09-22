package jev

import (
	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

// MockClient produces a deterministic, rule-based Evaluation without
// calling the real Jev API. It is used when TYPESAFE_API_KEY is not
// configured, so the CLI and evaluation pipeline remain runnable
// offline, and in tests.
type MockClient struct{}

// NewMockClient returns an Evaluator backed by simple ROI/occupancy
// margin rules.
func NewMockClient() *MockClient { return &MockClient{} }

// EvaluateProperty applies fixed thresholds (ROI >= 8%, occupancy
// margin >= 10 points over break-even) to derive a Go/Conditional/NoGo
// verdict, mirroring the questions asked of the real Jev model.
func (c *MockClient) EvaluateProperty(p property.Property, m airdna.MarketData, r finance.Result) (Evaluation, error) {
	occupancyMarginPoints := m.OccupancyRate*100 - r.BEPOccupancyRate*100
	roiSufficient := r.ROIPercent >= 8
	marginSafe := occupancyMarginPoints >= 10

	reasons := map[string]float64{
		"roi_sufficient":        boolToFloat(roiSufficient),
		"occupancy_margin_safe": boolToFloat(marginSafe),
	}

	switch {
	case roiSufficient && marginSafe:
		return Evaluation{Verdict: VerdictGo, Confidence: 0.9, Reasons: reasons}, nil
	case r.ROIPercent > 0:
		return Evaluation{Verdict: VerdictConditional, Confidence: 0.6, Reasons: reasons}, nil
	default:
		return Evaluation{Verdict: VerdictNoGo, Confidence: 0.85, Reasons: reasons}, nil
	}
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}
