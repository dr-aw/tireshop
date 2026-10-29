package httpapi

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"
	"tireshop/internal/catalogdata"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type productDTO struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	UnitPriceRub int64               `json:"unit_price_rub"`
	Available    bool                `json:"available"`
	Stock        int64               `json:"stock"`
	UnitLabel    string              `json:"unit_label"`
	Details      catalogdata.Details `json:"details"`
	Images       []catalogdata.Image `json:"images"`
}

var productColumns = `p.product_id::text,p.name,p.unit_price_rub,p.is_available,p.stock,` + catalogdata.Select("p") + `,
 (SELECT coalesce(jsonb_agg(jsonb_build_object('url',webp_path,'width',width,'height',height) ORDER BY position),'[]'::jsonb) FROM catalog.product_images i WHERE i.product_id=p.product_id)`

func readProduct(row pgx.Row) (productDTO, error) {
	var p productDTO
	var details, images []byte
	err := row.Scan(&p.ID, &p.Name, &p.UnitPriceRub, &p.Available, &p.Stock, &details, &images)
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(details, &p.Details); err != nil {
		return p, err
	}
	if err = json.Unmarshal(images, &p.Images); err != nil {
		return p, err
	}
	p.UnitLabel = p.Details.UnitLabel()
	return p, nil
}

// Catalog separates public reads from session-protected administration.
// Pagination bounds responses; unavailable products are omitted from public
// lists but can be read by ID.
func Catalog(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	health(mux, pool)
	catalogAdminDefaults(mux, pool)
	mux.HandleFunc("GET /api/products", func(w http.ResponseWriter, r *http.Request) {
		query, err := parseCatalogQuery(r.URL.Query(), false)
		if err != nil {
			fail(w, 400, "validation_error", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		var total int
		if err = pool.QueryRow(ctx, "SELECT count(*) FROM catalog.product_details p WHERE "+query.Where, query.Args...).Scan(&total); err != nil {
			fail(w, 503, "unavailable", "Каталог временно недоступен")
			return
		}
		args := append(query.Args, query.Limit, query.Offset)
		rows, err := pool.Query(ctx, "SELECT "+productColumns+" FROM catalog.product_details p WHERE "+query.Where+" ORDER BY "+query.Order+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)

		if err != nil {
			fail(w, 503, "unavailable", "Каталог временно недоступен")
			return
		}
		defer rows.Close()
		products := make([]productDTO, 0)
		for rows.Next() {
			p, err := readProduct(rows)
			if err != nil {
				fail(w, 503, "unavailable", "Каталог временно недоступен")
				return
			}
			p.Details.Location = ""
			p.Details.ReviewNotes = ""
			products = append(products, p)
		}
		if rows.Err() != nil {
			fail(w, 503, "unavailable", "Каталог временно недоступен")
			return
		}
		writeJSON(w, 200, struct {
			Items []productDTO `json:"items"`
			Total int          `json:"total"`
		}{products, total})
	})
	mux.HandleFunc("GET /api/products/filters", catalogFacets(pool, false))
	mux.HandleFunc("GET /api/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !uuid.MatchString(id) {
			fail(w, 400, "validation_error", "Некорректный идентификатор товара")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		p, err := readProduct(pool.QueryRow(ctx, `SELECT `+productColumns+`
			FROM catalog.product_details p WHERE product_id=$1::uuid`, id))
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "not_found", "Товар не найден")
			return
		}
		if err != nil {
			fail(w, 503, "unavailable", "Каталог временно недоступен")
			return
		}
		p.Details.Location = ""
		p.Details.ReviewNotes = ""
		writeJSON(w, 200, p)
	})
	return secure(mux)
}
