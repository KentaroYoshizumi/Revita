// Command revita is a minimal CLI prototype for Revita's core logic:
// take a candidate property, look up short-term-rental market data for
// its area, compute expected profitability, and ask an LLM to judge
// whether the property is worth investing in.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/estat"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/llm"
	"github.com/KentaroYoshizumi/Revita/internal/minpaku"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

func main() {
	address := flag.String("address", "東京都渋谷区神南1-1-1", "物件の住所")
	price := flag.Float64("price", 45000000, "物件購入価格（円）")
	rent := flag.Float64("rent", 150000, "月額費用（賃料・管理費等、円）")
	size := flag.Float64("size", 32.5, "広さ（平米）")
	capacity := flag.Int("capacity", 4, "定員（最大宿泊人数）")
	minpakuCSV := flag.String("minpaku-csv", "data/minpaku_todokede.csv", "民泊届出住宅数CSVのパス（競合物件数の実データ用、省略可）")
	flag.Parse()

	p := property.Property{
		Address:       *address,
		PurchasePrice: *price,
		MonthlyRent:   *rent,
		SizeSqm:       *size,
		Capacity:      *capacity,
	}

	fmt.Println("=== Revita: 物件収益性チェック ===")
	fmt.Printf("住所: %s\n", p.Address)
	fmt.Printf("購入価格: %.0f円 / 月額費用: %.0f円 / 広さ: %.1f㎡ / 定員: %d名\n\n",
		p.PurchasePrice, p.MonthlyRent, p.SizeSqm, p.Capacity)

	market, err := fetchMarketData(p.Address, *minpakuCSV)
	if err != nil {
		fmt.Fprintf(os.Stderr, "市場データの取得に失敗しました: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("--- 市場データ ---")
	fmt.Printf("平均日次単価(ADR): %.0f円 / 稼働率: %.1f%% / 競合物件数: %d件\n",
		market.ADR, market.OccupancyRate*100, market.CompetitorCount)
	fmt.Printf("出典: %s\n\n", market.DataSource)

	result := finance.Calculate(p, *market)

	fmt.Println("--- 想定収支 ---")
	fmt.Printf("想定月間収入: %.0f円\n", result.MonthlyRevenue)
	fmt.Printf("想定月間損益: %.0f円\n", result.MonthlyProfit)
	fmt.Printf("想定年間損益: %.0f円\n", result.AnnualProfit)
	fmt.Printf("年間ROI: %.2f%%\n\n", result.ROIPercent)

	fmt.Println("--- LLMによる判定 ---")
	verdict, err := llm.Judge(p, *market, result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "LLMによる判定に失敗しました: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(verdict)
}

// fetchMarketData tries Japan's free government statistics first
// (occupancy rate via e-Stat, competitor count via minpaku registration
// data), falling back to mock data when ESTAT_APP_ID/ESTAT_STATS_DATA_ID
// are not configured or the lookup otherwise fails.
func fetchMarketData(address, minpakuCSVPath string) (*airdna.MarketData, error) {
	appID := os.Getenv("ESTAT_APP_ID")
	statsDataID := os.Getenv("ESTAT_STATS_DATA_ID")

	if appID == "" || statsDataID == "" {
		return airdna.NewMockClient().GetMarketData(address)
	}

	counts, err := minpaku.LoadCSV(minpakuCSVPath)
	if err != nil {
		counts = nil // 未読込時は競合物件数のみモックにフォールバック
	}

	estatClient := estat.NewClient(appID, statsDataID)
	govClient := airdna.NewGovDataClient(estatClient, counts)

	market, err := govClient.GetMarketData(address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "政府統計データの取得に失敗したため、モックデータにフォールバックします: %v\n", err)
		return airdna.NewMockClient().GetMarketData(address)
	}
	return market, nil
}
