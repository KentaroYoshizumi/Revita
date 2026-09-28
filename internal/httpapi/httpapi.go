// Package httpapi exposes Revita's evaluation pipeline and billing
// flow over HTTP for the Next.js frontend: authenticated with Supabase
// JWTs, gated on an active subscription and a monthly execution limit.
package httpapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/authn"
	"github.com/KentaroYoshizumi/Revita/internal/billing"
	"github.com/KentaroYoshizumi/Revita/internal/compare"
	"github.com/KentaroYoshizumi/Revita/internal/history"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/pipeline"
	"github.com/KentaroYoshizumi/Revita/internal/property"
	"github.com/KentaroYoshizumi/Revita/internal/ratelimit"
)

const defaultHistoryListLimit = 50

// SubscriptionLookup is the subset of storage Server needs to check a
// user's subscription status.
type SubscriptionLookup interface {
	GetSubscriptionStatus(userID string) (string, error)
}

// MarketDataFetcher resolves market data for an address (mock or
// government-data backed; see cmd/revita for the equivalent CLI logic).
type MarketDataFetcher func(address string) (*airdna.MarketData, error)

// Server holds the dependencies shared by Revita's HTTP handlers.
type Server struct {
	Auth           *authn.Verifier
	Billing        *billing.Client
	Limiter        *ratelimit.Limiter
	Subscriptions  SubscriptionLookup
	FetchMarket    MarketDataFetcher
	Evaluator      jev.Evaluator
	GenerateReport pipeline.ReportGenerator
	// History persists evaluation runs so a user can list and compare
	// past properties (internal/compare). Reading/comparing history is
	// not gated by subscription or rate limit: it costs no Jev/LLM
	// call, since every underlying number was already computed (and
	// paid for) when the evaluation was first run.
	History history.Store
}

// evaluateRequest is the JSON body for POST /api/evaluate.
type evaluateRequest struct {
	Address       string  `json:"address"`
	PurchasePrice float64 `json:"purchase_price"`
	MonthlyRent   float64 `json:"monthly_rent"`
	SizeSqm       float64 `json:"size_sqm"`
	Capacity      int     `json:"capacity"`
	// BusinessType: "minpaku" (default, 180日/年規制あり) or "ryokan"
	// (規制なし). Empty is treated as "minpaku" by internal/finance.
	BusinessType string `json:"business_type"`
}

// Routes returns an http.Handler with all of Revita's API routes
// registered, wrapped with the auth middleware where required.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/api/evaluate", s.Auth.Middleware(http.HandlerFunc(s.handleEvaluate)))
	mux.Handle("/api/billing/checkout", s.Auth.Middleware(http.HandlerFunc(s.handleCheckout)))
	mux.Handle("/api/me", s.Auth.Middleware(http.HandlerFunc(s.handleMe)))
	mux.Handle("/api/evaluations", s.Auth.Middleware(http.HandlerFunc(s.handleListEvaluations)))
	mux.Handle("/api/evaluations/compare", s.Auth.Middleware(http.HandlerFunc(s.handleCompareEvaluations)))
	mux.HandleFunc("/api/billing/webhook", s.handleWebhook) // Stripe signs this itself; no bearer token

	return withCORS(mux)
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := authn.UserIDFromContext(r.Context())

	status, err := s.Subscriptions.GetSubscriptionStatus(userID)
	if err != nil {
		log.Printf("httpapi: failed to look up subscription for %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !billing.IsActive(status) {
		writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "subscription_required"})
		return
	}

	allowed, remaining, err := s.Limiter.Allow(userID)
	if err != nil {
		log.Printf("httpapi: rate limit check failed for %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !allowed {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "monthly_limit_exceeded"})
		return
	}

	var req evaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	p := property.Property{
		Address:       req.Address,
		PurchasePrice: req.PurchasePrice,
		MonthlyRent:   req.MonthlyRent,
		SizeSqm:       req.SizeSqm,
		Capacity:      req.Capacity,
		BusinessType:  property.BusinessType(req.BusinessType),
	}

	market, err := s.FetchMarket(p.Address)
	if err != nil {
		log.Printf("httpapi: failed to fetch market data for %q: %v", p.Address, err)
		http.Error(w, "failed to fetch market data", http.StatusInternalServerError)
		return
	}

	result, err := pipeline.Run(p, *market, s.Evaluator, s.GenerateReport)
	if err != nil {
		log.Printf("httpapi: pipeline failed: %v", err)
		http.Error(w, "evaluation failed", http.StatusInternalServerError)
		return
	}

	evaluationID := ""
	if s.History != nil {
		id, err := s.History.Save(history.Record{
			UserID:     userID,
			Property:   p,
			Market:     *market,
			Finance:    result.Finance,
			Evaluation: result.Evaluation,
			Report:     result.Report,
		})
		if err != nil {
			// Saving history is a nice-to-have, not the evaluation the
			// user is paying for right now: log and still return the
			// result rather than failing the whole request.
			log.Printf("httpapi: failed to save evaluation history for %s: %v", userID, err)
		}
		evaluationID = id
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":              evaluationID,
		"market":          market,
		"finance":         result.Finance,
		"evaluation":      result.Evaluation,
		"report_markdown": result.Report,
		"phase1_skipped":  result.Phase1Skipped,
		"phase2_skipped":  result.Phase2Skipped,
		"remaining_quota": remaining,
	})
}

// evaluationSummary is what handleListEvaluations returns per record:
// enough to render a history list with checkboxes, without shipping
// the full stored report_markdown for every row.
type evaluationSummary struct {
	ID        string    `json:"id"`
	Address   string    `json:"address"`
	Verdict   string    `json:"verdict"`
	NetYield  float64   `json:"net_yield_percent"`
	ROI       float64   `json:"roi_percent"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Server) handleListEvaluations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := authn.UserIDFromContext(r.Context())

	records, err := s.History.List(userID, defaultHistoryListLimit)
	if err != nil {
		log.Printf("httpapi: failed to list evaluations for %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	summaries := make([]evaluationSummary, 0, len(records))
	for _, rec := range records {
		summaries = append(summaries, evaluationSummary{
			ID:        rec.ID,
			Address:   rec.Property.Address,
			Verdict:   string(rec.Evaluation.Verdict),
			NetYield:  rec.Finance.NetYieldPercent,
			ROI:       rec.Finance.ROIPercent,
			CreatedAt: rec.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, summaries)
}

type compareRequest struct {
	IDs []string `json:"ids"`
}

func (s *Server) handleCompareEvaluations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := authn.UserIDFromContext(r.Context())

	var req compareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.IDs) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at_least_two_ids_required"})
		return
	}

	records, err := s.History.Get(userID, req.IDs)
	if err != nil {
		log.Printf("httpapi: failed to fetch evaluations for comparison for %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, compare.Compare(records))
}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := authn.UserIDFromContext(r.Context())
	email, _ := authn.EmailFromContext(r.Context())

	url, err := s.Billing.CreateCheckoutSession(userID, email)
	if err != nil {
		log.Printf("httpapi: failed to create checkout session for %s: %v", userID, err)
		http.Error(w, "failed to start checkout", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	const maxWebhookBodyBytes = 1 << 20 // 1MiB, Stripe's payloads are well under this
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	if err := s.Billing.HandleWebhook(payload, r.Header.Get("Stripe-Signature")); err != nil {
		log.Printf("httpapi: webhook handling failed: %v", err)
		http.Error(w, "webhook error", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := authn.UserIDFromContext(r.Context())
	email, _ := authn.EmailFromContext(r.Context())

	status, err := s.Subscriptions.GetSubscriptionStatus(userID)
	if err != nil {
		log.Printf("httpapi: failed to look up subscription for %s: %v", userID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":             userID,
		"email":               email,
		"subscription_status": status,
		"subscription_active": billing.IsActive(status),
	})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("httpapi: failed to encode response: %v", err)
	}
}
