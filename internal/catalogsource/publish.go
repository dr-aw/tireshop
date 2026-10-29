package catalogsource

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"tireshop/internal/catalogdata"
)

type Media struct {
	ProductID  string `json:"product_id"`
	SourcePath string `json:"source_path"`
	SHA256     string `json:"sha256"`
	WebPPath   string `json:"webp_path"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// Publish is an initial-load command owned by the catalog. It never updates a
// previously imported product: retries cannot restore stock after dispatch.
func Publish(ctx context.Context, pool *pgxpool.Pool, batch Batch, media []Media) (bool, error) {
	if len(batch.Products) == 0 || len(batch.Products) > 2000 {
		return false, fmt.Errorf("invalid batch size")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SET LOCAL statement_timeout='5s'; SET LOCAL lock_timeout='3s'`); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(62178001)`); err != nil {
		return false, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM catalog.import_batches WHERE source_hash=$1)`, batch.Hash).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	raw, err := json.Marshal(batch.Workbooks)
	if err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO catalog.import_batches(source_hash,source_workbooks,product_count) VALUES($1,$2::jsonb,$3)`, batch.Hash, raw, len(batch.Products)); err != nil {
		return false, err
	}
	ids := map[string]bool{}
	for _, p := range batch.Products {
		if err = p.Details.Validate(); err != nil {
			return false, err
		}
		ids[p.ID] = true
		if _, err = tx.Exec(ctx, `INSERT INTO catalog.products(product_id,name,unit_price_rub,stock,is_available) VALUES($1::uuid,$2,$3,$4,$5)`, p.ID, p.Name, p.Price, p.Stock, p.Available); err != nil {
			return false, fmt.Errorf("product %s conflicts; entire import rolled back: %w", p.Details.SKU, err)
		}
		d, e := json.Marshal(p.Details)
		if e != nil {
			return false, e
		}
		if _, err = tx.Exec(ctx, catalogdata.UpdateSQL(), p.ID, d); err != nil {
			return false, err
		}
		r, e := json.Marshal(p.Row)
		if e != nil {
			return false, e
		}
		if _, err = tx.Exec(ctx, `INSERT INTO catalog.source_rows(product_id,source_hash,source_file,sheet_name,row_number,raw_cells) VALUES($1::uuid,$2,$3,$4,$5,$6::jsonb)`, p.ID, batch.Hash, p.SourceFile, p.Sheet, p.Row.Number, r); err != nil {
			return false, err
		}
	}
	positions := map[string]int{}
	for _, m := range media {
		if !ids[m.ProductID] || !strings.HasPrefix(m.WebPPath, "/media/products/"+m.ProductID+"/") || strings.Contains(m.WebPPath, "..") || len(m.SHA256) != 64 || m.Width != 1280 || m.Height != 1280 {
			return false, fmt.Errorf("invalid media manifest")
		}
		pos := positions[m.ProductID]
		positions[m.ProductID]++
		if _, err = tx.Exec(ctx, `INSERT INTO catalog.product_images(product_id,position,source_path,source_sha256,webp_path,width,height) VALUES($1::uuid,$2,$3,$4,$5,$6,$7)`, m.ProductID, pos, m.SourcePath, m.SHA256, m.WebPPath, m.Width, m.Height); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}
