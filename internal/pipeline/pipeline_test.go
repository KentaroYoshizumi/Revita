package pipeline

import (
	"errors"
	"testing"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

type stubEvaluator struct {
	eval jev.Evaluation
	err  error
}

func (s stubEvaluator) EvaluateProperty(p property.Property, m airdna.MarketData, r finance.Result) (jev.Evaluation, error) {
	return s.eval, s.err
}

func sampleInputs() (property.Property, airdna.MarketData) {
	p := property.Property{Address: "東京都渋谷区神南1-1-1", PurchasePrice: 45000000, MonthlyRent: 150000}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.68}
	return p, m
}

func TestRun_NoGoSkipsPhase2(t *testing.T) {
	p, m := sampleInputs()
	evaluator := stubEvaluator{eval: jev.Evaluation{Verdict: jev.VerdictNoGo, Confidence: 0.9}}

	called := false
	fakeGenerate := func(property.Property, airdna.MarketData, finance.Result, jev.Evaluation) (string, error) {
		called = true
		return "SHOULD NOT BE CALLED", nil
	}

	result, err := Run(p, m, evaluator, fakeGenerate)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if called {
		t.Error("Phase 2 report generator was called on a NoGo verdict, expected it to be skipped")
	}
	if !result.Phase2Skipped {
		t.Error("Phase2Skipped = false, want true")
	}
	if result.Report == "" {
		t.Error("expected a local summary Report even when Phase 2 is skipped")
	}
}

func TestRun_GoCallsPhase2(t *testing.T) {
	p, m := sampleInputs()
	evaluator := stubEvaluator{eval: jev.Evaluation{Verdict: jev.VerdictGo, Confidence: 0.9}}

	called := false
	fakeGenerate := func(property.Property, airdna.MarketData, finance.Result, jev.Evaluation) (string, error) {
		called = true
		return "# detailed report", nil
	}

	result, err := Run(p, m, evaluator, fakeGenerate)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Error("Phase 2 report generator was not called on a Go verdict")
	}
	if result.Phase2Skipped {
		t.Error("Phase2Skipped = true, want false")
	}
	if result.Report != "# detailed report" {
		t.Errorf("Report = %q, want %q", result.Report, "# detailed report")
	}
}

func TestRun_ConditionalCallsPhase2(t *testing.T) {
	p, m := sampleInputs()
	evaluator := stubEvaluator{eval: jev.Evaluation{Verdict: jev.VerdictConditional, Confidence: 0.6}}

	called := false
	fakeGenerate := func(property.Property, airdna.MarketData, finance.Result, jev.Evaluation) (string, error) {
		called = true
		return "# conditional report", nil
	}

	if _, err := Run(p, m, evaluator, fakeGenerate); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Error("Phase 2 report generator was not called on a Conditional verdict")
	}
}

func TestRun_Phase1Error(t *testing.T) {
	p, m := sampleInputs()
	evaluator := stubEvaluator{err: errors.New("jev unreachable")}

	if _, err := Run(p, m, evaluator, nil); err == nil {
		t.Fatal("expected an error when Phase 1 fails")
	}
}
