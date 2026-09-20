// Package airdna provides access to short-term-rental market data.
//
// The real AirDNA API is a paid third-party service, so this package
// currently ships only a mock client that returns data shaped like a real
// AirDNA "market" response. Swapping in a real HTTP-backed client later
// only requires implementing the Client interface below.
package airdna

import (
	"encoding/json"
	"fmt"
)

// MarketData mirrors the subset of an AirDNA market-metrics response that
// Revita's ROI calculation needs.
type MarketData struct {
	Address         string  `json:"address"`
	ADR             float64 `json:"average_daily_rate"` // 平均日次単価（円）
	OccupancyRate   float64 `json:"occupancy_rate"`     // 稼働率（0.0〜1.0）
	RevPAR          float64 `json:"rev_par"`            // 部屋あたり収益（円/日）
	CompetitorCount int     `json:"competitor_count"`   // 周辺の競合物件数
	Currency        string  `json:"currency"`
}

// Client fetches market data for a given address.
type Client interface {
	GetMarketData(address string) (*MarketData, error)
}

// MockClient returns dummy data shaped like a real AirDNA API response.
// It is used until a real API key/integration is wired up.
type MockClient struct{}

// NewMockClient returns a Client backed by static dummy data.
func NewMockClient() *MockClient {
	return &MockClient{}
}

// GetMarketData returns dummy market data for the given address, decoded
// from a JSON payload shaped like AirDNA's real response format.
func (c *MockClient) GetMarketData(address string) (*MarketData, error) {
	raw := fmt.Sprintf(`{
		"address": %q,
		"average_daily_rate": 18500,
		"occupancy_rate": 0.68,
		"rev_par": 12580,
		"competitor_count": 24,
		"currency": "JPY"
	}`, address)

	var data MarketData
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	return &data, nil
}
