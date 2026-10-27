// Package httpapi translates HTTP/JSON into application operations. Database
// transactions and business rules remain in their existing packages.
package httpapi

import (
	"context"
	"encoding/json/v2"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON marshals before sending headers so serialization cannot leave an
// apparently successful response containing a partial JSON document.
func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, "Response encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func fail(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, struct {
		Error apiError `json:"error"`
	}{apiError{code, message}})
}

// Health is process liveness; readiness additionally checks the service's own
// restricted connection. Neither endpoint exposes connection settings.
func health(mux *http.ServeMux, pool *pgxpool.Pool) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if pool.Ping(ctx) != nil {
			fail(w, 503, "unavailable", "Сервис временно недоступен")
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ready"})
	})
}

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

type bucket struct {
	tokens float64
	last   time.Time
}
type limiter struct {
	mu      sync.Mutex
	clients map[string]bucket
}

// Allow uses only the socket peer, never an untrusted X-Forwarded-For header.
// This bounded, process-local limiter is suitable for the local first slice.
// A deployed proxy must have an explicit trusted-proxy policy before per-client
// limits can distinguish visitors behind it.
func (l *limiter) allow(remote string) bool {
	ip, _, err := net.SplitHostPort(remote)
	if err != nil {
		ip = remote
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b, exists := l.clients[ip]
	if !exists {
		if len(l.clients) >= 1024 {
			for key, old := range l.clients {
				if now.Sub(old.last) > 5*time.Minute {
					delete(l.clients, key)
				}
			}
			if len(l.clients) >= 1024 {
				return false
			}
		}
		b = bucket{tokens: 30, last: now}
	}
	b.tokens = min(30, b.tokens+now.Sub(b.last).Seconds())
	b.last = now
	allowed := b.tokens >= 1
	if allowed {
		b.tokens--
	}
	l.clients[ip] = b
	return allowed
}
