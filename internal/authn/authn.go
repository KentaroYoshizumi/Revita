// Package authn verifies Supabase Auth JWTs on incoming API requests.
//
// Supabase Auth issues HS256-signed JWTs to signed-in users. The Go
// backend verifies them using the project's JWT secret (Supabase
// dashboard: Project Settings -> API -> JWT Secret), rather than
// re-implementing sign-in itself — sign-up/sign-in stays in the Next.js
// frontend via the Supabase JS client.
package authn

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	userIDContextKey contextKey = "revita.userID"
	emailContextKey  contextKey = "revita.email"
)

// Verifier checks a Supabase Auth JWT.
type Verifier struct {
	JWTSecret string
}

// NewVerifier returns a Verifier for the given Supabase project JWT
// secret.
func NewVerifier(jwtSecret string) *Verifier {
	return &Verifier{JWTSecret: jwtSecret}
}

func (v *Verifier) parseClaims(tokenString string) (jwt.MapClaims, error) {
	if v.JWTSecret == "" {
		return nil, fmt.Errorf("authn: jwt secret is not configured")
	}

	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("authn: unexpected signing method %v", t.Header["alg"])
		}
		return []byte(v.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, fmt.Errorf("authn: invalid token: %w", err)
	}
	return claims, nil
}

// VerifyUserID parses and validates tokenString, returning the user ID
// from its "sub" claim.
func (v *Verifier) VerifyUserID(tokenString string) (string, error) {
	claims, err := v.parseClaims(tokenString)
	if err != nil {
		return "", err
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", fmt.Errorf("authn: token missing sub claim")
	}
	return sub, nil
}

// Verify parses and validates tokenString, returning the user ID
// ("sub" claim) and email ("email" claim, empty if absent).
func (v *Verifier) Verify(tokenString string) (userID, email string, err error) {
	claims, err := v.parseClaims(tokenString)
	if err != nil {
		return "", "", err
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return "", "", fmt.Errorf("authn: token missing sub claim")
	}
	email, _ = claims["email"].(string)
	return sub, email, nil
}

// Middleware extracts the "Authorization: Bearer <token>" header,
// verifies it, and stores the user ID and email in the request context
// for downstream handlers. Requests without a valid token receive 401.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		userID, email, err := v.Verify(token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		ctx = context.WithValue(ctx, emailContextKey, email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext returns the authenticated user ID stored by
// Middleware, if any.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)
	return userID, ok
}

// EmailFromContext returns the authenticated user's email stored by
// Middleware, if any. It may be empty even when ok is true, if the JWT
// carried no "email" claim.
func EmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(emailContextKey).(string)
	return email, ok
}
