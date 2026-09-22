package authn

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-jwt-secret"

func signToken(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}
	return signed
}

func TestVerifyUserID_Valid(t *testing.T) {
	v := NewVerifier(testSecret)
	token := signToken(t, testSecret, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	userID, err := v.VerifyUserID(token)
	if err != nil {
		t.Fatalf("VerifyUserID returned error: %v", err)
	}
	if userID != "user-123" {
		t.Errorf("userID = %q, want %q", userID, "user-123")
	}
}

func TestVerifyUserID_WrongSecret(t *testing.T) {
	v := NewVerifier(testSecret)
	token := signToken(t, "wrong-secret", jwt.MapClaims{"sub": "user-123"})

	if _, err := v.VerifyUserID(token); err == nil {
		t.Fatal("expected an error for a token signed with the wrong secret")
	}
}

func TestVerifyUserID_Expired(t *testing.T) {
	v := NewVerifier(testSecret)
	token := signToken(t, testSecret, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	if _, err := v.VerifyUserID(token); err == nil {
		t.Fatal("expected an error for an expired token")
	}
}

func TestVerifyUserID_MissingSub(t *testing.T) {
	v := NewVerifier(testSecret)
	token := signToken(t, testSecret, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()})

	if _, err := v.VerifyUserID(token); err == nil {
		t.Fatal("expected an error for a token missing the sub claim")
	}
}

func TestMiddleware(t *testing.T) {
	v := NewVerifier(testSecret)
	token := signToken(t, testSecret, jwt.MapClaims{
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	var gotUserID string
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotUserID != "user-123" {
		t.Errorf("userID in context = %q, want %q", gotUserID, "user-123")
	}
}

func TestMiddleware_MissingHeader(t *testing.T) {
	v := NewVerifier(testSecret)
	handler := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without a valid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}
