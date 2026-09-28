// Package property defines the input data for a candidate real estate
// property that Revita evaluates.
package property

// BusinessType identifies which legal basis the property will operate
// under, since that determines whether Japan's 180-day annual operating
// cap applies (see internal/finance).
type BusinessType string

const (
	// BusinessTypeMinpaku is 住宅宿泊事業法（民泊）registration: capped
	// at 180 operating days per year nationwide. This is the default,
	// since it's the license most individual short-term-rental
	// operators use.
	BusinessTypeMinpaku BusinessType = "minpaku"
	// BusinessTypeRyokan is a full 旅館業許可 (hotel/inn business
	// license): no annual operating-day cap, but a heavier regulatory
	// and facility burden to obtain.
	BusinessTypeRyokan BusinessType = "ryokan"
)

// Property represents the minimal set of information needed to run a
// profitability check on a short-term rental (Airbnb-style) candidate.
type Property struct {
	Address       string  // 住所
	PurchasePrice float64 // 物件購入価格（円）
	MonthlyRent   float64 // 月額費用（賃料・管理費など、円）
	SizeSqm       float64 // 広さ（平米）
	Capacity      int     // 定員（最大宿泊人数）
	// BusinessType selects which operating-day rules apply. The zero
	// value ("") is treated as BusinessTypeMinpaku by internal/finance.
	BusinessType BusinessType
}
