package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func catalogSnapshot(pool *pgxpool.Pool) http.HandlerFunc {
	gate := make(chan struct{}, 1)
	return func(w http.ResponseWriter, r *http.Request) {
		select {
		case gate <- struct{}{}:
			defer func() { <-gate }()
		default:
			fail(w, 429, "busy", "Повторите экспорт позже")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		// One statement (including correlated image reads) uses one PostgreSQL snapshot.
		rows, err := pool.Query(ctx, "SELECT "+productColumns+" FROM catalog.product_details p WHERE p.is_available ORDER BY "+publicCatalogOrder+" LIMIT 10001")
		if err != nil {
			fail(w, 503, "unavailable", "Экспорт временно недоступен")
			return
		}
		defer rows.Close()
		items := make([]productDTO, 0)
		for rows.Next() {
			p, err := readProduct(rows)
			if err != nil {
				fail(w, 503, "unavailable", "Экспорт временно недоступен")
				return
			}
			p.Details.Location = ""
			p.Details.ReviewNotes = ""
			items = append(items, p)
		}
		if rows.Err() != nil || len(items) > 10000 {
			fail(w, 503, "unavailable", "Экспорт неполон или превышает лимит")
			return
		}
		data, err := json.Marshal(items)
		if err != nil {
			fail(w, 503, "unavailable", "Экспорт временно недоступен")
			return
		}
		hash := sha256.Sum256(data)
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, struct {
			SchemaVersion int          `json:"schema_version"`
			Version       string       `json:"version"`
			GeneratedAt   string       `json:"generated_at"`
			Items         []productDTO `json:"items"`
		}{1, hex.EncodeToString(hash[:]), time.Now().UTC().Format(time.RFC3339), items})
	}
}
