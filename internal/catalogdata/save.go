package catalogdata

import (
	"context"
	"encoding/json/v2"
	"github.com/jackc/pgx/v5"
	"strings"
)

var commonFields = []string{"sku", "kind", "description", "brand", "model", "production_year", "location", "condition", "units_per_sale", "review_notes"}

// Save runs inside the caller's transaction, after its optimistic-version lock.
// The FK ensures that attributes can never belong to the wrong product kind.
func Save(ctx context.Context, tx pgx.Tx, id string, d Details) error {
	if err := d.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	// Remove the old subtype before changing kind; rollback restores both on error.
	if _, err = tx.Exec(ctx, `DELETE FROM catalog.tyre_specs WHERE product_id=$1 AND $2<>'tyre'`, id, d.Kind); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM catalog.wheel_specs WHERE product_id=$1 AND $2<>'wheel'`, id, d.Kind); err != nil {
		return err
	}
	parts := []string{}
	for _, field := range commonFields {
		parts = append(parts, field+"=d."+field)
	}
	if _, err = tx.Exec(ctx, `UPDATE catalog.products p SET `+strings.Join(parts, ",")+` FROM jsonb_populate_record(NULL::catalog.product_details,$2::jsonb) d WHERE p.product_id=$1::uuid`, id, raw); err != nil {
		return err
	}
	if d.Kind == "tyre" {
		_, err = tx.Exec(ctx, `INSERT INTO catalog.tyre_specs(product_id,size,season,tread_percent,load_index,speed_index) VALUES($1,$2,$3,$4,$5,$6)
   ON CONFLICT(product_id) DO UPDATE SET size=excluded.size,season=excluded.season,tread_percent=excluded.tread_percent,load_index=excluded.load_index,speed_index=excluded.speed_index`, id, d.Size, d.Season, d.TreadPercent, d.LoadIndex, d.SpeedIndex)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO catalog.wheel_specs(product_id,marking,diameter_inches,width_inches,offset_mm,pcd,wheel_type,center_bore_mm) VALUES($1,$2,$3,$4,$5,$6,$7,$8)
   ON CONFLICT(product_id) DO UPDATE SET marking=excluded.marking,diameter_inches=excluded.diameter_inches,width_inches=excluded.width_inches,offset_mm=excluded.offset_mm,pcd=excluded.pcd,wheel_type=excluded.wheel_type,center_bore_mm=excluded.center_bore_mm`, id, d.Size, d.Diameter, d.Width, d.WheelOffset, d.PCD, d.WheelType, d.CenterBore)
	}
	return err
}
