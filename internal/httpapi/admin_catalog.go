package httpapi

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"tireshop/internal/requests"
	"tireshop/internal/requests/postgres"
)

type adminProduct struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	UnitPriceRub *int64 `json:"unit_price_rub"`
	Available    *bool  `json:"available"`
	Stock        *int64 `json:"stock"`
	Version      int64  `json:"version"`
}

// The catalog knows only the session introspection HTTP contract. The requests
// service owns users and sessions; an outage denies catalog administration.
func catalogAdmin(mux *http.ServeMux, pool *pgxpool.Pool, sessionURL string, origins []string) {
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	protect := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
			defer cancel()
			r = r.WithContext(ctx)
			cookie, err := r.Cookie(sessionCookie)
			if err != nil {
				fail(w, 401, "unauthorized", "Войдите в админ-панель")
				return
			}
			check, err := http.NewRequestWithContext(ctx, "GET", sessionURL, nil)
			if err != nil {
				fail(w, 503, "unavailable", "Проверка сессии недоступна")
				return
			}
			check.AddCookie(cookie)
			resp, err := client.Do(check)
			if err != nil {
				fail(w, 503, "unavailable", "Проверка сессии недоступна")
				return
			}
			resp.Body.Close()
			if resp.StatusCode == 401 {
				fail(w, 401, "unauthorized", "Войдите в админ-панель")
				return
			}
			if resp.StatusCode != 200 {
				fail(w, 503, "unavailable", "Проверка сессии недоступна")
				return
			}
			if r.Method != "GET" && !allowedMutation(w, r, origins) {
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("GET /api/admin/products", protect(func(w http.ResponseWriter, r *http.Request) {
		offset, ok := pageOffset(w, r)
		if !ok {
			return
		}
		rows, err := pool.Query(r.Context(), `SELECT product_id::text,name,unit_price_rub,is_available,stock,version FROM catalog.products ORDER BY product_id LIMIT 50 OFFSET $1`, offset)
		if err != nil {
			adminError(w, err)
			return
		}
		defer rows.Close()
		items := []adminProduct{}
		for rows.Next() {
			var p adminProduct
			if err := rows.Scan(&p.ID, &p.Name, &p.UnitPriceRub, &p.Available, &p.Stock, &p.Version); err != nil {
				adminError(w, err)
				return
			}
			items = append(items, p)
		}
		if rows.Err() != nil {
			adminError(w, rows.Err())
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	}))
	save := protect(func(w http.ResponseWriter, r *http.Request) {
		var p adminProduct
		if !adminBody(w, r, &p) {
			return
		}
		p.Name = strings.TrimSpace(p.Name)
		if utf8.RuneCountInString(p.Name) < 1 || utf8.RuneCountInString(p.Name) > 300 || p.UnitPriceRub == nil || *p.UnitPriceRub < 0 || *p.UnitPriceRub > requests.MaxAmount || p.Stock == nil || *p.Stock < 0 || *p.Stock > requests.MaxAmount || p.Available == nil {
			fail(w, 422, "validation_error", "Укажите название, цену, целый остаток и доступность")
			return
		}
		if r.Method == "POST" {
			// The client supplies a UUID, so a retry cannot create another product.
			if !uuid.MatchString(p.ID) {
				fail(w, 422, "validation_error", "Нужен идентификатор товара")
				return
			}
			tag, err := pool.Exec(r.Context(), `INSERT INTO catalog.products(product_id,name,unit_price_rub,is_available,stock) VALUES($1::uuid,$2,$3,$4,$5) ON CONFLICT(product_id) DO NOTHING`, p.ID, p.Name, *p.UnitPriceRub, *p.Available, *p.Stock)
			if err != nil {
				adminError(w, err)
				return
			}
			if tag.RowsAffected() != 1 {
				adminError(w, postgres.ErrVersion)
				return
			}
			p.Version = 1
			writeJSON(w, 201, p)
			return
		}
		id := r.PathValue("id")
		if !uuid.MatchString(id) || p.Version < 1 {
			fail(w, 422, "validation_error", "Проверьте идентификатор и версию")
			return
		}
		tag, err := pool.Exec(r.Context(), `UPDATE catalog.products SET name=$2,unit_price_rub=$3,is_available=$4,stock=$5,version=version+1 WHERE product_id=$1::uuid AND version=$6`, id, p.Name, *p.UnitPriceRub, *p.Available, *p.Stock, p.Version)
		if err != nil {
			adminError(w, err)
			return
		}
		if tag.RowsAffected() != 1 {
			adminError(w, postgres.ErrVersion)
			return
		}
		p.ID = id
		p.Version++
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("POST /api/admin/products", save)
	mux.HandleFunc("PUT /api/admin/products/{id}", save)
}
func catalogAdminDefaults(mux *http.ServeMux, pool *pgxpool.Pool) {
	url := os.Getenv("ADMIN_SESSION_URL")
	if url == "" {
		url = "http://127.0.0.1:8082/api/admin/session"
	}
	origins := os.Getenv("APP_ORIGINS")
	if origins == "" {
		origins = "http://localhost:5173,http://127.0.0.1:5173"
	}
	catalogAdmin(mux, pool, url, strings.Split(origins, ","))
}
