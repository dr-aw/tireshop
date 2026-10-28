package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"tireshop/internal/adminauth"
)

const sessionCookie = "tireshop_admin"

type adminGate struct {
	pool    *pgxpool.Pool
	origins []string
}

func adminBody(w http.ResponseWriter, r *http.Request, v any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		fail(w, 415, "json_required", "Ожидается JSON")
		return false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		fail(w, 413, "body_limit", "Слишком большой запрос")
		return false
	}
	if json.Unmarshal(body, v, json.RejectUnknownMembers(true)) != nil {
		fail(w, 400, "invalid_json", "Проверьте формат данных")
		return false
	}
	return true
}
func allowedMutation(w http.ResponseWriter, r *http.Request, origins []string) bool {
	for _, origin := range origins {
		if origin != "" && r.Header.Get("Origin") == origin && r.Header.Get("X-Admin-Request") == "1" {
			return true
		}
	}
	fail(w, 403, "csrf", "Источник запроса не разрешён")
	return false
}
func (a adminGate) actor(ctx context.Context, r *http.Request) (string, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || len(cookie.Value) != 64 {
		return "", pgx.ErrNoRows
	}
	hash := sha256.Sum256([]byte(cookie.Value))
	var actor string
	err = a.pool.QueryRow(ctx, `SELECT u.username FROM requests.admin_sessions s JOIN requests.admin_users u USING(username)
 WHERE token_hash=$1 AND expires_at>now() AND active AND role='admin'`, hash[:]).Scan(&actor)
	return actor, err
}
func (a adminGate) protect(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		r = r.WithContext(ctx)
		actor, err := a.actor(ctx, r)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 401, "unauthorized", "Войдите в админ-панель")
			return
		}
		if err != nil {
			fail(w, 503, "unavailable", "Проверка сессии временно недоступна")
			return
		}
		if r.Method != "GET" && !allowedMutation(w, r, a.origins) {
			return
		}
		next(w, r, actor)
	}
}
func cookieFor(value string, age int) *http.Cookie {
	return &http.Cookie{Name: sessionCookie, Value: value, Path: "/api", HttpOnly: true, Secure: os.Getenv("ADMIN_LOCAL_HTTP") != "1", SameSite: http.SameSiteStrictMode, MaxAge: age}
}

// Login has both a fixed-window peer/account budget and a memory-work cap.
// Counters are process-local; a production proxy needs a shared policy.
type loginWindow struct {
	since time.Time
	count int
}

func (a adminGate) routes(mux *http.ServeMux) {
	var mu sync.Mutex
	windows := map[string]loginWindow{}
	work := make(chan struct{}, 2)
	dummy := adminauth.Hash("unused-account-timing-placeholder")
	mux.HandleFunc("POST /api/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if !allowedMutation(w, r, a.origins) {
			return
		}
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !adminBody(w, r, &in) {
			return
		}
		in.Username = strings.ToLower(strings.TrimSpace(in.Username))
		if len(in.Username) > 80 || len(in.Password) > 1024 {
			fail(w, 401, "invalid_login", "Неверный логин или пароль")
			return
		}
		// Socket peer only; do not trust forwarded headers.
		peer := r.RemoteAddr
		if i := strings.LastIndex(peer, ":"); i >= 0 {
			peer = peer[:i]
		}
		mu.Lock()
		now := time.Now()
		for key, v := range windows {
			if now.Sub(v.since) >= time.Minute {
				delete(windows, key)
			}
		}
		allowed := len(windows) < 2046
		if allowed {
			for _, key := range []string{"ip:" + peer, "user:" + in.Username} {
				v := windows[key]
				if v.count == 0 {
					v.since = now
				}
				v.count++
				windows[key] = v
				allowed = allowed && v.count <= 5
			}
		}
		mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			fail(w, 429, "rate_limited", "Слишком много попыток. Подождите минуту")
			return
		}
		select {
		case work <- struct{}{}:
			defer func() { <-work }()
		default:
			fail(w, 429, "rate_limited", "Вход занят. Повторите позже")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		// Lock the account through session creation so a concurrent password reset
		// cannot leave a fresh session authenticated with the previous password.
		tx, err := a.pool.Begin(ctx)
		if err != nil {
			fail(w, 503, "unavailable", "Вход временно недоступен")
			return
		}
		defer tx.Rollback(ctx)
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock_shared(hashtextextended($1,0))`, "admin:"+in.Username); err != nil {
			fail(w, 503, "unavailable", "Вход временно недоступен")
			return
		}
		var hash string
		err = tx.QueryRow(ctx, `SELECT password_hash FROM requests.admin_users WHERE username=$1 AND active AND role='admin'`, in.Username).Scan(&hash)
		missing := errors.Is(err, pgx.ErrNoRows)
		if err != nil && !missing {
			fail(w, 503, "unavailable", "Вход временно недоступен")
			return
		}
		if missing {
			hash = dummy
		}
		if !adminauth.Verify(hash, in.Password) || missing {
			fail(w, 401, "invalid_login", "Неверный логин или пароль")
			return
		}
		raw := make([]byte, 32)
		if _, err = rand.Read(raw); err != nil {
			fail(w, 503, "unavailable", "Вход временно недоступен")
			return
		}
		token := hex.EncodeToString(raw)
		digest := sha256.Sum256([]byte(token))
		_, err = tx.Exec(ctx, `DELETE FROM requests.admin_sessions WHERE expires_at<=now()`)
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO requests.admin_sessions(token_hash,username,expires_at) VALUES($1,$2,now()+interval '72 hours')`, digest[:], in.Username)
		}
		if err != nil || tx.Commit(ctx) != nil {
			fail(w, 503, "unavailable", "Вход временно недоступен")
			return
		}
		http.SetCookie(w, cookieFor(token, 72*60*60))
		writeJSON(w, 200, map[string]string{"username": in.Username})
	})
	mux.HandleFunc("GET /api/admin/session", a.protect(func(w http.ResponseWriter, r *http.Request, actor string) {
		writeJSON(w, 200, map[string]string{"username": actor})
	}))
	mux.HandleFunc("POST /api/admin/logout", a.protect(func(w http.ResponseWriter, r *http.Request, actor string) {
		cookie, _ := r.Cookie(sessionCookie)
		digest := sha256.Sum256([]byte(cookie.Value))
		if _, err := a.pool.Exec(r.Context(), `DELETE FROM requests.admin_sessions WHERE token_hash=$1`, digest[:]); err != nil {
			fail(w, 503, "unavailable", "Выход не подтверждён")
			return
		}
		http.SetCookie(w, cookieFor("", -1))
		writeJSON(w, 200, map[string]bool{"ok": true})
	}))
}
