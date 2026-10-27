package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type productDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	UnitPriceRub int64  `json:"unit_price_rub"`
	Available    bool   `json:"available"`
}

// Catalog exposes public product data only. Pagination bounds the query and
// response; unavailable products are omitted from lists but can be read by ID.
func Catalog(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	health(mux, pool)
	mux.HandleFunc("GET /api/products", func(w http.ResponseWriter, r *http.Request) {
		limit, offset := 24, 0
		var err error
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
		}
		if err != nil || limit < 1 || limit > 100 {
			fail(w, 400, "validation_error", "Некорректный размер страницы")
			return
		}
		if raw := r.URL.Query().Get("offset"); raw != "" {
			offset, err = strconv.Atoi(raw)
		}
		if err != nil || offset < 0 || offset > 100000 {
			fail(w, 400, "validation_error", "Некорректный номер страницы")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		rows, err := pool.Query(ctx, `SELECT product_id::text, name, unit_price_rub, is_available
			FROM catalog.products WHERE is_available ORDER BY product_id LIMIT $1 OFFSET $2`, limit, offset)
		if err != nil {
			fail(w, 503, "unavailable", "Каталог временно недоступен")
			return
		}
		defer rows.Close()
		products := make([]productDTO, 0)
		for rows.Next() {
			var p productDTO
			if err := rows.Scan(&p.ID, &p.Name, &p.UnitPriceRub, &p.Available); err != nil {
				fail(w, 503, "unavailable", "Каталог временно недоступен")
				return
			}
			products = append(products, p)
		}
		if rows.Err() != nil {
			fail(w, 503, "unavailable", "Каталог временно недоступен")
			return
		}
		writeJSON(w, 200, struct {
			Items []productDTO `json:"items"`
		}{products})
	})
	mux.HandleFunc("GET /api/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !uuid.MatchString(id) {
			fail(w, 400, "validation_error", "Некорректный идентификатор товара")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		var p productDTO
		err := pool.QueryRow(ctx, `SELECT product_id::text, name, unit_price_rub, is_available
			FROM catalog.products WHERE product_id=$1::uuid`, id).Scan(&p.ID, &p.Name, &p.UnitPriceRub, &p.Available)
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "not_found", "Товар не найден")
			return
		}
		if err != nil {
			fail(w, 503, "unavailable", "Каталог временно недоступен")
			return
		}
		writeJSON(w, 200, p)
	})
	return secure(mux)
}
