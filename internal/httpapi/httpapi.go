// Package httpapi exposes Revita's evaluation pipeline and billing
// flow over HTTP for the Next.js frontend: authenticated with Supabase
// JWTs, gated on an active subscription and a monthly execution limit.
package httpapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/authn"
	"github.com/KentaroYoshizumi/Revita/internal/billing"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/pipeline"
	"github.com/KentaroYoshizumi/Revita/internal/property"
	"github.com/KentaroYoshizumi/Revita/internal/ratelimit"
)

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
}

// evaluateRequest is the JSON body for POST /api/evaluate.
type evaluateRequest struct {
	Address       string  `json:"address"`
	PurchasePrice float64 `json:"purchase_price"`
	MonthlyRent   float64 `json:"monthly_rent"`
	SizeSqm       float64 `json:"size_sqm"`
	Capacity      int     `json:"capacity"`
}

// Routes returns an http.Handler with all of Revita's API routes
// registered, wrapped with the auth middleware where required.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("/api/evaluate", s.Auth.Middleware(http.HandlerFunc(s.handleEvaluate)))
	mux.Handle("/api/billing/checkout", s.Auth.Middleware(http.HandlerFunc(s.handleCheckout)))
	mux.Handle("/api/me", s.Auth.Middleware(http.HandlerFunc(s.handleMe)))
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"market":          market,
		"finance":         result.Finance,
		"evaluation":      result.Evaluation,
		"report_markdown": result.Report,
		"phase2_skipped":  result.Phase2Skipped,
		"remaining_quota": remaining,
	})
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
