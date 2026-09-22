package httpapi

import (
	"net/http"
	"os"
)

// withCORS allows the Next.js frontend (a different origin in local
// development, and typically a different deployment in production) to
// call this API. The allowed origin is read from FRONTEND_ORIGIN so it
// can be locked down per environment; it defaults to allowing any
// origin, which is fine for local development but should be set
// explicitly in production.
func withCORS(next http.Handler) http.Handler {
	origin := os.Getenv("FRONTEND_ORIGIN")
	if origin == "" {
		origin = "*"
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
