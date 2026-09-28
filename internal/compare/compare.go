// Package compare ranks a user's previously saved property evaluations
// against each other. Every number involved (ROI, yield, break-even
// margin) is already computed by internal/finance and stored in
// internal/history, so comparing them needs no further Jev or LLM
// call — it's a pure, free, deterministic ranking.
package compare

import (
	"fmt"
	"sort"

	"github.com/KentaroYoshizumi/Revita/internal/history"
)

// RankingEntry is one candidate's position in a ranking.
type RankingEntry struct {
	ID      string  `json:"id"`
	Address string  `json:"address"`
	Value   float64 `json:"value"`
}

// Result is a full comparison across a set of saved evaluations.
type Result struct {
	ByNetYield    []RankingEntry `json:"by_net_yield"`   // 実質利回りが高い順
	ByBEPMargin   []RankingEntry `json:"by_bep_margin"`  // 損益分岐稼働率に対する余裕（ポイント）が大きい順
	RecommendedID string         `json:"recommended_id"` // 総合的に最も推奨される候補のID（空文字なら該当なし）
	Narrative     string         `json:"narrative"`      // ルールベースの日本語コメント
}

// Compare ranks records by 実質利回り (net yield) and by break-even
// occupancy margin, and recommends the highest-net-yield candidate
// among those that are legally achievable under the minpaku 180-day
// cap (see finance.Result.LegallyAchievable) — a candidate that can
// never break even is excluded from the recommendation regardless of
// how good its numbers look otherwise.
func Compare(records []history.Record) Result {
	byNetYield := make([]RankingEntry, len(records))
	byBEPMargin := make([]RankingEntry, len(records))

	for i, rec := range records {
		byNetYield[i] = RankingEntry{ID: rec.ID, Address: rec.Property.Address, Value: rec.Finance.NetYieldPercent}
		marginPoints := (rec.Finance.EffectiveOccupancyRate - rec.Finance.BEPOccupancyRate) * 100
		byBEPMargin[i] = RankingEntry{ID: rec.ID, Address: rec.Property.Address, Value: marginPoints}
	}

	sort.Slice(byNetYield, func(i, j int) bool { return byNetYield[i].Value > byNetYield[j].Value })
	sort.Slice(byBEPMargin, func(i, j int) bool { return byBEPMargin[i].Value > byBEPMargin[j].Value })

	recommended := recommend(records)

	return Result{
		ByNetYield:    byNetYield,
		ByBEPMargin:   byBEPMargin,
		RecommendedID: recommended,
		Narrative:     narrative(records, byNetYield, byBEPMargin, recommended),
	}
}

// recommend returns the ID of the highest 実質利回り candidate among
// those that are legally achievable, or "" if none are.
func recommend(records []history.Record) string {
	byID := make(map[string]history.Record, len(records))
	for _, rec := range records {
		byID[rec.ID] = rec
	}

	best := ""
	bestYield := 0.0
	first := true
	for id, rec := range byID {
		if !rec.Finance.LegallyAchievable {
			continue
		}
		if first || rec.Finance.NetYieldPercent > bestYield {
			best = id
			bestYield = rec.Finance.NetYieldPercent
			first = false
		}
	}
	return best
}

func narrative(records []history.Record, byNetYield, byBEPMargin []RankingEntry, recommendedID string) string {
	if len(records) == 0 {
		return "比較対象がありません。"
	}

	byID := make(map[string]history.Record, len(records))
	for _, rec := range records {
		byID[rec.ID] = rec
	}

	out := fmt.Sprintf("実質利回りが最も高いのは%s（%.2f%%）です。", byNetYield[0].Address, byNetYield[0].Value)
	out += fmt.Sprintf(" 損益分岐稼働率に対する余裕が最も大きいのは%s（%.1fポイント）です。", byBEPMargin[0].Address, byBEPMargin[0].Value)

	if recommendedID == "" {
		out += " ただし、いずれの物件も民泊180日/年規制の範囲内では損益分岐点に到達できないため、総合的な推奨候補はありません。"
		return out
	}
	out += fmt.Sprintf(" 総合的に最も推奨できるのは%sです。", byID[recommendedID].Property.Address)

	var excluded []string
	for _, rec := range records {
		if !rec.Finance.LegallyAchievable {
			excluded = append(excluded, rec.Property.Address)
		}
	}
	if len(excluded) > 0 {
		out += fmt.Sprintf(" なお、次の物件は民泊180日/年規制の範囲内では損益分岐点に到達できないため、比較から除外しています: %v", excluded)
	}

	return out
}
