// Package pipeline runs Revita's 2-phase property evaluation:
//
//  1. Go pre-computes exact ROI/BEP/yield figures (internal/finance).
//  2. Phase 1: Jev, a fast/low-cost judgment-only model, turns those
//     figures plus market data into a structured Go/Conditional/NoGo
//     verdict with a confidence score (internal/jev).
//  3. Phase 2: a large LLM turns the verdict into a detailed Markdown
//     advisory report for the user (internal/llm) — skipped on a NoGo
//     verdict to save API cost, replaced by a short local summary.
package pipeline

import (
	"fmt"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/llm"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

// ReportGenerator produces the Phase 2 Markdown report. It matches
// llm.GenerateReport's signature but is a separate func type so tests
// can substitute a fake instead of calling a real LLM API.
type ReportGenerator func(p property.Property, m airdna.MarketData, r finance.Result, eval jev.Evaluation) (string, error)

// Result is the outcome of running the full pipeline.
type Result struct {
	Finance       finance.Result
	Evaluation    jev.Evaluation
	Report        string // Markdown
	Phase2Skipped bool   // true if Phase 2 (LLM) generation was skipped
}

// Run executes both phases: Phase 1 (Jev) always runs; Phase 2 (the LLM
// report) is skipped when Jev's verdict is NoGo, using a local summary
// instead to save cost.
func Run(p property.Property, m airdna.MarketData, evaluator jev.Evaluator, generateReport ReportGenerator) (Result, error) {
	r := finance.Calculate(p, m)

	eval, err := evaluator.EvaluateProperty(p, m, r)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: phase 1 (jev) failed: %w", err)
	}

	if eval.Verdict == jev.VerdictNoGo {
		return Result{
			Finance:       r,
			Evaluation:    eval,
			Report:        llm.SimpleSummary(eval, r),
			Phase2Skipped: true,
		}, nil
	}

	report, err := generateReport(p, m, r, eval)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: phase 2 (llm) failed: %w", err)
	}

	return Result{Finance: r, Evaluation: eval, Report: report}, nil
}
