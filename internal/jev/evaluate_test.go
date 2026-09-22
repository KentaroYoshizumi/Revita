package jev

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

func samplePropertyAndData() (property.Property, airdna.MarketData, finance.Result) {
	p := property.Property{Address: "東京都渋谷区神南1-1-1", PurchasePrice: 45000000, MonthlyRent: 150000}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.68, CompetitorCount: 24, DataSource: "モック"}
	r := finance.Calculate(p, m)
	return p, m, r
}

func TestClient_EvaluateProperty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"model": "jev-1.13.0",
			"answers": {
				"verdict": {"type": "choice", "choice": "Go", "confidence": 0.88},
				"roi_sufficient": {"type": "noul", "noul": 0.9},
				"occupancy_margin_safe": {"type": "noul", "noul": 0.8}
			},
			"usage": {"input_tokens": 200, "output_tokens": 0}
		}`)
	}))
	defer server.Close()

	c := &Client{APIKey: "test-key", HTTPClient: server.Client(), testBaseURL: server.URL}
	p, m, r := samplePropertyAndData()

	eval, err := c.EvaluateProperty(p, m, r)
	if err != nil {
		t.Fatalf("EvaluateProperty returned error: %v", err)
	}
	if eval.Verdict != VerdictGo {
		t.Errorf("Verdict = %v, want %v", eval.Verdict, VerdictGo)
	}
	if eval.Confidence != 0.88 {
		t.Errorf("Confidence = %v, want 0.88", eval.Confidence)
	}
	if eval.Reasons["roi_sufficient"] != 0.9 {
		t.Errorf("Reasons[roi_sufficient] = %v, want 0.9", eval.Reasons["roi_sufficient"])
	}
}

func TestClient_EvaluateProperty_UnexpectedVerdict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"answers": {"verdict": {"type": "choice", "choice": "Maybe"}}}`)
	}))
	defer server.Close()

	c := &Client{APIKey: "test-key", HTTPClient: server.Client(), testBaseURL: server.URL}
	p, m, r := samplePropertyAndData()

	if _, err := c.EvaluateProperty(p, m, r); err == nil {
		t.Fatal("expected an error for an unexpected verdict value")
	}
}

func TestMockClient_EvaluateProperty_Go(t *testing.T) {
	c := NewMockClient()
	p := property.Property{PurchasePrice: 20000000, MonthlyRent: 50000}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.68}
	r := finance.Calculate(p, m)

	eval, err := c.EvaluateProperty(p, m, r)
	if err != nil {
		t.Fatalf("EvaluateProperty returned error: %v", err)
	}
	if eval.Verdict != VerdictGo {
		t.Errorf("Verdict = %v, want %v (high ROI, low BEP)", eval.Verdict, VerdictGo)
	}
}

func TestMockClient_EvaluateProperty_NoGo(t *testing.T) {
	c := NewMockClient()
	p := property.Property{PurchasePrice: 45000000, MonthlyRent: 400000}
	m := airdna.MarketData{ADR: 18500, OccupancyRate: 0.68}
	r := finance.Calculate(p, m)

	eval, err := c.EvaluateProperty(p, m, r)
	if err != nil {
		t.Fatalf("EvaluateProperty returned error: %v", err)
	}
	if eval.Verdict != VerdictNoGo {
		t.Errorf("Verdict = %v, want %v (negative profit)", eval.Verdict, VerdictNoGo)
	}
}
