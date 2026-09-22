// Command server runs Revita as a SaaS HTTP API: Supabase Auth-gated,
// billed via Stripe (¥300/month), and rate-limited per user per month.
// It is the backend for the Next.js frontend under web/.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/authn"
	"github.com/KentaroYoshizumi/Revita/internal/billing"
	"github.com/KentaroYoshizumi/Revita/internal/db"
	"github.com/KentaroYoshizumi/Revita/internal/estat"
	"github.com/KentaroYoshizumi/Revita/internal/httpapi"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/llm"
	"github.com/KentaroYoshizumi/Revita/internal/minpaku"
	"github.com/KentaroYoshizumi/Revita/internal/ratelimit"
)

const defaultMonthlyLimit = 30

func main() {
	dsn := requireEnv("DATABASE_URL")
	jwtSecret := requireEnv("SUPABASE_JWT_SECRET")
	stripeKey := requireEnv("STRIPE_SECRET_KEY")
	stripePriceID := requireEnv("STRIPE_PRICE_ID")
	stripeWebhookSecret := requireEnv("STRIPE_WEBHOOK_SECRET")
	checkoutSuccessURL := requireEnv("CHECKOUT_SUCCESS_URL")
	checkoutCancelURL := requireEnv("CHECKOUT_CANCEL_URL")

	limit := defaultMonthlyLimit
	if v := os.Getenv("MONTHLY_EXECUTION_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("invalid MONTHLY_EXECUTION_LIMIT %q: %v", v, err)
		}
		limit = n
	}

	database, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	billingClient := billing.NewClient(stripeKey, stripePriceID, stripeWebhookSecret, checkoutSuccessURL, checkoutCancelURL, database)

	server := &httpapi.Server{
		Auth:           authn.NewVerifier(jwtSecret),
		Billing:        billingClient,
		Limiter:        ratelimit.NewLimiter(database, limit),
		Subscriptions:  database,
		FetchMarket:    fetchMarketData,
		Evaluator:      evaluator(),
		GenerateReport: llm.GenerateReport,
	}

	addr := ":" + envOrDefault("PORT", "8080")
	log.Printf("revita server listening on %s (monthly limit: %d)", addr, limit)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

// fetchMarketData mirrors cmd/revita's CLI logic: prefer Japan's free
// government statistics, falling back to mock data when unconfigured
// or unreachable.
func fetchMarketData(address string) (*airdna.MarketData, error) {
	appID := os.Getenv("ESTAT_APP_ID")
	statsDataID := os.Getenv("ESTAT_STATS_DATA_ID")

	if appID == "" || statsDataID == "" {
		return airdna.NewMockClient().GetMarketData(address)
	}

	counts, err := minpaku.LoadCSV(envOrDefault("MINPAKU_CSV_PATH", "data/minpaku_todokede.csv"))
	if err != nil {
		counts = nil
	}

	estatClient := estat.NewClient(appID, statsDataID)
	govClient := airdna.NewGovDataClient(estatClient, counts)

	market, err := govClient.GetMarketData(address)
	if err != nil {
		log.Printf("government market data lookup failed, falling back to mock: %v", err)
		return airdna.NewMockClient().GetMarketData(address)
	}
	return market, nil
}

// evaluator mirrors cmd/revita's CLI logic: the real Jev client when
// TYPESAFE_API_KEY is configured, otherwise the rule-based mock.
func evaluator() jev.Evaluator {
	if apiKey := os.Getenv("TYPESAFE_API_KEY"); apiKey != "" {
		return jev.NewClient(apiKey)
	}
	return jev.NewMockClient()
}

func requireEnv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("missing required environment variable %s", name)
	}
	return v
}

func envOrDefault(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
