package airdna

import (
	"fmt"

	"github.com/KentaroYoshizumi/Revita/internal/minpaku"
	"github.com/KentaroYoshizumi/Revita/internal/prefecture"
)

// mockADR is used for the average daily rate until a real short-term-rental
// pricing data source (e.g. a paid market data API) is wired in — Japan's
// free government statistics do not publish a nightly-rate equivalent.
const mockADR = 18500.0

// occupancyLookup is the subset of *estat.Client that GovDataClient
// depends on, so tests can substitute a stub instead of making real
// HTTP calls.
type occupancyLookup interface {
	GetOccupancyRate(areaCode string) (float64, error)
}

// GovDataClient builds MarketData from Japan's free government
// statistics instead of AirDNA: room occupancy rate from the Japan
// Tourism Agency's 宿泊旅行統計調査 (via e-Stat), and competitor count
// from its minpaku registration figures. The average daily rate has no
// free government equivalent, so it stays a fixed placeholder.
type GovDataClient struct {
	Estat         occupancyLookup
	MinpakuCounts minpaku.Counts // nil if not loaded
}

// NewGovDataClient returns a Client backed by free Japanese government
// statistics. minpakuCounts may be nil, in which case CompetitorCount
// falls back to a fixed placeholder.
func NewGovDataClient(estatClient occupancyLookup, minpakuCounts minpaku.Counts) *GovDataClient {
	return &GovDataClient{Estat: estatClient, MinpakuCounts: minpakuCounts}
}

// GetMarketData resolves the address to a prefecture and returns market
// data built from government statistics (occupancy rate, competitor
// count) plus a mock ADR.
func (c *GovDataClient) GetMarketData(address string) (*MarketData, error) {
	pref, found := prefecture.FromAddress(address)
	if !found {
		return nil, fmt.Errorf("govdata: could not identify a prefecture in address %q", address)
	}

	occupancy, err := c.Estat.GetOccupancyRate(pref.Code + "000")
	if err != nil {
		return nil, fmt.Errorf("govdata: occupancy rate lookup failed: %w", err)
	}

	competitorCount := 24
	competitorSource := "モック（民泊届出データ未読込）"
	if c.MinpakuCounts != nil {
		if n, ok := c.MinpakuCounts[pref.Name]; ok {
			competitorCount = n
			competitorSource = "観光庁 住宅宿泊事業（民泊）届出データ"
		}
	}

	return &MarketData{
		Address:         address,
		ADR:             mockADR,
		OccupancyRate:   occupancy,
		RevPAR:          mockADR * occupancy,
		CompetitorCount: competitorCount,
		Currency:        "JPY",
		DataSource: fmt.Sprintf(
			"稼働率: 観光庁「宿泊旅行統計調査」(e-Stat) / 競合物件数: %s / ADR: モック",
			competitorSource,
		),
	}, nil
}
