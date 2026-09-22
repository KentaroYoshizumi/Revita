package jev

import (
	"fmt"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

// Verdict is Jev's coarse Phase-1 pre-screening judgment.
type Verdict string

const (
	VerdictGo          Verdict = "Go"
	VerdictConditional Verdict = "Conditional"
	VerdictNoGo        Verdict = "NoGo"
)

// Evaluation is Revita's structured Phase-1 result, derived from Jev's
// typed answers.
type Evaluation struct {
	Verdict    Verdict
	Confidence float64            // verdict選択の確度（0.0-1.0）
	Reasons    map[string]float64 // 判定軸ごとの確度スコア（noul/score値）
}

// Evaluator lets the pipeline depend on either the real Jev-backed
// Client or the offline MockClient interchangeably.
type Evaluator interface {
	EvaluateProperty(p property.Property, m airdna.MarketData, r finance.Result) (Evaluation, error)
}

// buildState renders the Go-calculated numbers and market data into the
// plain-text "state" that Jev judges against.
func buildState(p property.Property, m airdna.MarketData, r finance.Result) string {
	return fmt.Sprintf(
		"物件: %s / 購入価格 %.0f円 / 月額費用 %.0f円\n"+
			"市場データ(出典: %s): ADR %.0f円 / 稼働率 %.1f%% / 競合物件数 %d件\n"+
			"収支計算結果: 想定月間損益 %.0f円 / 年間ROI %.2f%% / 表面利回り %.2f%% / 実質利回り %.2f%% / 損益分岐稼働率 %.1f%%",
		p.Address, p.PurchasePrice, p.MonthlyRent,
		m.DataSource, m.ADR, m.OccupancyRate*100, m.CompetitorCount,
		r.MonthlyProfit, r.ROIPercent, r.GrossYieldPercent, r.NetYieldPercent, r.BEPOccupancyRate*100,
	)
}

func evaluationQuestions() map[string]Question {
	return map[string]Question{
		"verdict": {
			Type: TypeChoice,
			Instructions: "この短期賃貸物件への投資は「Go」「Conditional」「NoGo」のどれに該当するか。" +
				"ROIが十分高く、稼働率が損益分岐稼働率に対して十分な余裕を持つ場合はGo。" +
				"ROIか稼働率の余裕のいずれかが不十分な場合はConditional。" +
				"年間ROIが低い、または市場の稼働率が損益分岐稼働率を下回っている場合はNoGo。",
			Criteria: map[string]string{
				"Go":          "ROIが高く、稼働率にも十分な余裕がある",
				"Conditional": "ROI・稼働率の余裕のいずれかが十分でない",
				"NoGo":        "ROIが低い、または市場の稼働率が損益分岐稼働率を下回っている",
			},
		},
		"roi_sufficient": {
			Type:         TypeNoul,
			Instructions: "この物件の年間ROIは、一般的な不動産投資の基準（目安8%以上）を満たしているか。",
		},
		"occupancy_margin_safe": {
			Type:         TypeNoul,
			Instructions: "市場の稼働率は、この物件の損益分岐稼働率に対して十分な余裕（目安10ポイント以上）を持っているか。",
		},
	}
}

// EvaluateProperty runs Jev's Phase-1 screening on Go-calculated
// numbers and market data.
func (c *Client) EvaluateProperty(p property.Property, m airdna.MarketData, r finance.Result) (Evaluation, error) {
	state := buildState(p, m, r)
	answers, err := c.Evaluate(state, evaluationQuestions())
	if err != nil {
		return Evaluation{}, err
	}
	return toEvaluation(answers)
}

func toEvaluation(answers map[string]Answer) (Evaluation, error) {
	verdictAnswer, ok := answers["verdict"]
	if !ok || verdictAnswer.Choice == "" {
		return Evaluation{}, fmt.Errorf("jev: missing verdict answer")
	}

	verdict := Verdict(verdictAnswer.Choice)
	switch verdict {
	case VerdictGo, VerdictConditional, VerdictNoGo:
	default:
		return Evaluation{}, fmt.Errorf("jev: unexpected verdict value %q", verdictAnswer.Choice)
	}

	confidence := 0.0
	if verdictAnswer.Confidence != nil {
		confidence = *verdictAnswer.Confidence
	}

	reasons := map[string]float64{}
	for id, a := range answers {
		if id == "verdict" {
			continue
		}
		switch {
		case a.Noul != nil:
			reasons[id] = *a.Noul
		case a.Score != nil:
			reasons[id] = *a.Score
		}
	}

	return Evaluation{Verdict: verdict, Confidence: confidence, Reasons: reasons}, nil
}
