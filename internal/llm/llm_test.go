package llm

import (
	"strings"
	"testing"

	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
)

func TestSimpleSummary_NoGoExplainsReasons(t *testing.T) {
	eval := jev.Evaluation{
		Verdict:    jev.VerdictNoGo,
		Confidence: 0.85,
		Reasons: map[string]float64{
			"roi_sufficient":        0,
			"occupancy_margin_safe": 0,
		},
	}
	r := finance.Result{ROIPercent: -1.5, BEPOccupancyRate: 0.9}

	summary := SimpleSummary(eval, r)

	if !strings.Contains(summary, "判定理由") {
		t.Error("summary should contain a 判定理由 section")
	}
	if !strings.Contains(summary, "投資基準（目安8%以上）を下回っています") {
		t.Errorf("summary should explain the low ROI, got:\n%s", summary)
	}
	if !strings.Contains(summary, "余裕が小さい") {
		t.Errorf("summary should explain the thin occupancy margin, got:\n%s", summary)
	}
	if !strings.Contains(summary, "コスト削減のためスキップ") {
		t.Error("NoGo summary should mention Phase 2 was skipped for cost reasons")
	}
}

func TestSimpleSummary_GoExplainsReasonsPositively(t *testing.T) {
	eval := jev.Evaluation{
		Verdict:    jev.VerdictGo,
		Confidence: 0.9,
		Reasons: map[string]float64{
			"roi_sufficient":        1,
			"occupancy_margin_safe": 1,
		},
	}
	r := finance.Result{ROIPercent: 12, BEPOccupancyRate: 0.3}

	summary := SimpleSummary(eval, r)

	if !strings.Contains(summary, "投資基準（目安8%以上）を満たしています") {
		t.Errorf("summary should explain the sufficient ROI, got:\n%s", summary)
	}
	if !strings.Contains(summary, "十分な余裕があります") {
		t.Errorf("summary should explain the healthy occupancy margin, got:\n%s", summary)
	}
}

func TestExplainReasons_UnknownAxisFallsBackToGeneric(t *testing.T) {
	eval := jev.Evaluation{Reasons: map[string]float64{"some_new_axis": 0.42}}
	got := explainReasons(eval, finance.Result{})

	if !strings.Contains(got, "some_new_axis: 0.42") {
		t.Errorf("expected a generic fallback line for an unrecognized axis, got: %q", got)
	}
}

func TestExplainReasons_LegallyAchievableNotDuplicated(t *testing.T) {
	eval := jev.Evaluation{Reasons: map[string]float64{"legally_achievable": 0}}
	got := explainReasons(eval, finance.Result{BEPOccupancyRate: 0.7})

	if strings.Count(got, "legally_achievable") != 0 {
		t.Errorf("legally_achievable should only produce the dedicated sentence, not also the generic fallback line; got:\n%s", got)
	}
	if !strings.Contains(got, "住宅宿泊事業法") {
		t.Errorf("expected the dedicated legal-cap explanation, got:\n%s", got)
	}
}

func TestExplainReasons_NoReasons(t *testing.T) {
	got := explainReasons(jev.Evaluation{}, finance.Result{})
	if got != "(判定根拠のスコアがありません)" {
		t.Errorf("got %q, want the no-reasons placeholder", got)
	}
}
