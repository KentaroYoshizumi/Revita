package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/authn"
	"github.com/KentaroYoshizumi/Revita/internal/billing"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/property"
	"github.com/KentaroYoshizumi/Revita/internal/ratelimit"
)

const testJWTSecret = "test-secret"

func signTestToken(t *testing.T, userID, email string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": userID, "email": email, "exp": time.Now().Add(time.Hour).Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return token
}

type fakeSubscriptions struct {
	status string
}

func (f fakeSubscriptions) GetSubscriptionStatus(userID string) (string, error) {
	return f.status, nil
}

type fakeUsageStore struct {
	counts map[string]int
}

func (f *fakeUsageStore) IncrementAndGet(userID, yearMonth string) (int, error) {
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[userID]++
	return f.counts[userID], nil
}

func newTestServer(subStatus string, limit int) *Server {
	return &Server{
		Auth:          authn.NewVerifier(testJWTSecret),
		Limiter:       ratelimit.NewLimiter(&fakeUsageStore{}, limit),
		Subscriptions: fakeSubscriptions{status: subStatus},
		FetchMarket: func(address string) (*airdna.MarketData, error) {
			return &airdna.MarketData{ADR: 18500, OccupancyRate: 0.68, DataSource: "test"}, nil
		},
		Evaluator: jev.NewMockClient(),
		GenerateReport: func(p property.Property, m airdna.MarketData, r finance.Result, eval jev.Evaluation) (string, error) {
			return "# test report", nil
		},
	}
}

func TestHandleEvaluate_RequiresSubscription(t *testing.T) {
	s := newTestServer("none", 10)
	token := signTestToken(t, "user-1", "user1@example.com")

	body, _ := json.Marshal(evaluateRequest{Address: "東京都渋谷区神南1-1-1", PurchasePrice: 20000000, MonthlyRent: 50000})
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("status = %d, want %d; body=%s", rec.Code, http.StatusPaymentRequired, rec.Body.String())
	}
}

func TestHandleEvaluate_ActiveSubscriptionSucceeds(t *testing.T) {
	s := newTestServer("active", 10)
	token := signTestToken(t, "user-1", "user1@example.com")

	body, _ := json.Marshal(evaluateRequest{Address: "東京都渋谷区神南1-1-1", PurchasePrice: 20000000, MonthlyRent: 50000})
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHandleEvaluate_MonthlyLimitExceeded(t *testing.T) {
	s := newTestServer("active", 1)
	token := signTestToken(t, "user-1", "user1@example.com")
	body, _ := json.Marshal(evaluateRequest{Address: "東京都渋谷区神南1-1-1", PurchasePrice: 20000000, MonthlyRent: 50000})

	// First call consumes the only allowed slot.
	req1 := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first call status = %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second call status = %d, want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

func TestHandleEvaluate_MissingToken(t *testing.T) {
	s := newTestServer("active", 10)
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestHandleMe(t *testing.T) {
	s := newTestServer("active", 10)
	token := signTestToken(t, "user-1", "user1@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got["subscription_active"] != true {
		t.Errorf("subscription_active = %v, want true", got["subscription_active"])
	}
}

func TestHandleWebhook_InvalidSignatureRejected(t *testing.T) {
	s := newTestServer("active", 10)
	s.Billing = &billing.Client{WebhookSecret: "whsec_test"}

	req := httptest.NewRequest(http.MethodPost, "/api/billing/webhook", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Stripe-Signature", "t=1,v1=invalid")
	rec := httptest.NewRecorder()

	s.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
