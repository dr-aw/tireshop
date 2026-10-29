package httpapi

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"tireshop/internal/catalogdata"
	"tireshop/internal/requests"
	"tireshop/internal/requests/postgres"
)

type adminProduct struct {
	ID           string               `json:"id"`
	Name         string               `json:"name"`
	UnitPriceRub *int64               `json:"unit_price_rub"`
	Available    *bool                `json:"available"`
	Stock        *int64               `json:"stock"`
	Version      int64                `json:"version"`
	Details      *catalogdata.Details `json:"details,omitempty"`
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
		kind := r.URL.Query().Get("kind")
		if kind != "" && kind != "tyre" && kind != "wheel" {
			fail(w, 400, "validation_error", "Некорректная категория")
			return
		}
		rows, err := pool.Query(r.Context(), `SELECT product_id::text,name,unit_price_rub,is_available,stock,version,`+catalogdata.Select("p")+` FROM catalog.products p WHERE ($2 = '' OR kind = $2) ORDER BY product_id LIMIT 50 OFFSET $1`, offset, kind)
		if err != nil {
			adminError(w, err)
			return
		}
		defer rows.Close()
		items := []adminProduct{}
		for rows.Next() {
			var p adminProduct
			var raw []byte
			if err := rows.Scan(&p.ID, &p.Name, &p.UnitPriceRub, &p.Available, &p.Stock, &p.Version, &raw); err != nil {
				adminError(w, err)
				return
			}
			if err := json.Unmarshal(raw, &p.Details); err != nil {
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
		if p.Details != nil && p.Details.Validate() != nil {
			fail(w, 422, "validation_error", "Проверьте характеристики товара")
			return
		}
		if utf8.RuneCountInString(p.Name) < 1 || utf8.RuneCountInString(p.Name) > 300 || p.UnitPriceRub == nil || *p.UnitPriceRub < 0 || *p.UnitPriceRub > requests.MaxAmount || p.Stock == nil || *p.Stock < 0 || *p.Stock > requests.MaxAmount || p.Available == nil {
			fail(w, 422, "validation_error", "Укажите название, цену, целый остаток и доступность")
			return
		}
		tx, err := pool.Begin(r.Context())
		if err != nil {
			adminError(w, err)
			return
		}
		defer tx.Rollback(r.Context())
		finish := func(id string) bool {
			if p.Details != nil {
				raw, e := json.Marshal(p.Details)
				if e != nil {
					adminError(w, e)
					return false
				}
				if _, e = tx.Exec(r.Context(), catalogdata.UpdateSQL(), id, raw); e != nil {
					adminError(w, e)
					return false
				}
			}
			if e := tx.Commit(r.Context()); e != nil {
				adminError(w, e)
				return false
			}
			return true
		}
		if r.Method == "POST" {
			// The client supplies a UUID, so a retry cannot create another product.
			if !uuid.MatchString(p.ID) {
				fail(w, 422, "validation_error", "Нужен идентификатор товара")
				return
			}
			tag, err := tx.Exec(r.Context(), `INSERT INTO catalog.products(product_id,name,unit_price_rub,is_available,stock) VALUES($1::uuid,$2,$3,$4,$5) ON CONFLICT(product_id) DO NOTHING`, p.ID, p.Name, *p.UnitPriceRub, *p.Available, *p.Stock)
			if err != nil {
				adminError(w, err)
				return
			}
			if tag.RowsAffected() != 1 {
				adminError(w, postgres.ErrVersion)
				return
			}
			p.Version = 1
			if !finish(p.ID) {
				return
			}
			writeJSON(w, 201, p)
			return
		}
		id := r.PathValue("id")
		if !uuid.MatchString(id) || p.Version < 1 {
			fail(w, 422, "validation_error", "Проверьте идентификатор и версию")
			return
		}
		var units any
		if p.Details != nil {
			units = p.Details.UnitsPerSale
		}
		tag, err := tx.Exec(r.Context(), `UPDATE catalog.products SET name=$2,unit_price_rub=$3,is_available=$4,stock=$5,version=version+1 WHERE product_id=$1::uuid AND version=$6 AND ($7::integer IS NULL OR units_per_sale=$7)`, id, p.Name, *p.UnitPriceRub, *p.Available, *p.Stock, p.Version, units)
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
		if !finish(id) {
			return
		}
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
