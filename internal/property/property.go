// Package property defines the input data for a candidate real estate
// property that Revita evaluates.
package property

// Property represents the minimal set of information needed to run a
// profitability check on a short-term rental (Airbnb-style) candidate.
type Property struct {
	Address       string  // 住所
	PurchasePrice float64 // 物件購入価格（円）
	MonthlyRent   float64 // 月額費用（賃料・管理費など、円）
	SizeSqm       float64 // 広さ（平米）
	Capacity      int     // 定員（最大宿泊人数）
}
