package airdna

import (
	"errors"
	"testing"

	"github.com/KentaroYoshizumi/Revita/internal/minpaku"
)

type stubEstat struct {
	rate float64
	err  error
}

func (s stubEstat) GetOccupancyRate(areaCode string) (float64, error) {
	return s.rate, s.err
}

func TestGovDataClient_GetMarketData(t *testing.T) {
	client := NewGovDataClient(stubEstat{rate: 0.72}, minpaku.Counts{"東京都": 4321})

	data, err := client.GetMarketData("東京都渋谷区神南1-1-1")
	if err != nil {
		t.Fatalf("GetMarketData returned error: %v", err)
	}
	if data.OccupancyRate != 0.72 {
		t.Errorf("OccupancyRate = %v, want 0.72", data.OccupancyRate)
	}
	if data.CompetitorCount != 4321 {
		t.Errorf("CompetitorCount = %v, want 4321", data.CompetitorCount)
	}
	if data.ADR != mockADR {
		t.Errorf("ADR = %v, want mockADR (%v)", data.ADR, mockADR)
	}
}

func TestGovDataClient_MinpakuFallback(t *testing.T) {
	client := NewGovDataClient(stubEstat{rate: 0.5}, nil)

	data, err := client.GetMarketData("福岡県福岡市中央区天神1-1")
	if err != nil {
		t.Fatalf("GetMarketData returned error: %v", err)
	}
	if data.CompetitorCount != 24 {
		t.Errorf("CompetitorCount = %v, want fallback value 24", data.CompetitorCount)
	}
}

func TestGovDataClient_UnknownPrefecture(t *testing.T) {
	client := NewGovDataClient(stubEstat{rate: 0.5}, nil)

	if _, err := client.GetMarketData("存在しない住所"); err == nil {
		t.Fatal("expected an error when no prefecture can be identified")
	}
}

func TestGovDataClient_EstatError(t *testing.T) {
	client := NewGovDataClient(stubEstat{err: errors.New("boom")}, nil)

	if _, err := client.GetMarketData("東京都渋谷区神南1-1-1"); err == nil {
		t.Fatal("expected an error when the estat lookup fails")
	}
}
