// Package pipeline runs Revita's 2-phase property evaluation:
//
//  1. Go pre-computes exact ROI/BEP/yield figures (internal/finance),
//     including whether break-even is even legally possible within
//     Japan's 180-day/year minpaku operating cap.
//  2. Phase 1: Jev, a fast/low-cost judgment-only model, turns those
//     figures plus market data into a structured Go/Conditional/NoGo
//     verdict with a confidence score (internal/jev) — skipped
//     entirely when Go's own numbers already prove the property can
//     never break even under the legal cap, since no model judgment is
//     needed to know that.
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
	Phase1Skipped bool   // true if Phase 1 (Jev) was skipped (legally unachievable — see finance.Result.LegallyAchievable)
	Phase2Skipped bool   // true if Phase 2 (LLM) generation was skipped
}

// Run executes both phases: Phase 1 (Jev) always runs unless Go's own
// numbers already show the property can never break even under the
// minpaku 180-day/year cap, in which case both phases are skipped and
// an automatic NoGo is returned without spending on either API. Phase 2
// (the LLM report) is skipped when the verdict is NoGo, using a local
// summary instead to save cost.
func Run(p property.Property, m airdna.MarketData, evaluator jev.Evaluator, generateReport ReportGenerator) (Result, error) {
	r := finance.Calculate(p, m)

	if !r.LegallyAchievable {
		eval := jev.Evaluation{
			Verdict:    jev.VerdictNoGo,
			Confidence: 1.0,
			Reasons:    map[string]float64{"legally_achievable": 0},
		}
		return Result{
			Finance:       r,
			Evaluation:    eval,
			Report:        llm.SimpleSummary(eval, r),
			Phase1Skipped: true,
			Phase2Skipped: true,
		}, nil
	}

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
