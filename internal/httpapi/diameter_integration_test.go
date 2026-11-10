//go:build integration

package httpapi

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http/httptest"
	"strings"
	"testing"
	"tireshop/internal/catalogdata"
)

func TestDiameterConstraints(t *testing.T) {
	pool, _ := apiPools(t)
	db := adminDB(t)
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM catalog.products WHERE name='Diameter bounds fixture'`)
	})
	var tyre, wheel string
	for kind, id := range map[string]*string{"tyre": &tyre, "wheel": &wheel} {
		if err := pool.QueryRow(t.Context(), `INSERT INTO catalog.products(name,kind,unit_price_rub) VALUES('Diameter bounds fixture',$1,100) RETURNING product_id::text`, kind).Scan(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO catalog.tyre_specs(product_id,size) VALUES($1,'205/85R16')`, tyre); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO catalog.wheel_specs(product_id,diameter_inches) VALUES($1,17.5)`, wheel); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"11", "25", "17.5"} {
		if _, err := pool.Exec(t.Context(), `UPDATE catalog.tyre_specs SET size=$1 WHERE product_id=$2`, "210/90R"+value, tyre); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(t.Context(), `UPDATE catalog.wheel_specs SET diameter_inches=$1 WHERE product_id=$2`, value, wheel); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"10", "10.99", "25.01", "26"} {
		for _, kind := range []string{"tyre", "wheel"} {
			var err error
			if kind == "tyre" {
				_, err = pool.Exec(t.Context(), `UPDATE catalog.tyre_specs SET size=$1 WHERE product_id=$2`, "210/90R"+value, tyre)
			} else {
				_, err = pool.Exec(t.Context(), `UPDATE catalog.wheel_specs SET diameter_inches=$1 WHERE product_id=$2`, value, wheel)
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
				t.Fatalf("%s %s: %v", kind, value, err)
			}
			response := httptest.NewRecorder()
			adminError(response, err)
			if response.Code != 422 || !strings.Contains(response.Body.String(), "11 до 25") {
				t.Fatal(response.Body.String())
			}
		}
	}
	// The unchanged editor Save path derives all fields and rolls back a rejected edit.
	for _, size := range []string{"210/90R17", "210/90R26"} {
		tx, err := pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		err = catalogdata.Save(t.Context(), tx, tyre, catalogdata.Details{Kind: "tyre", Size: size, UnitsPerSale: 1})
		if size == "210/90R26" {
			if err == nil {
				t.Fatal("accepted invalid editor input")
			}
			tx.Rollback(t.Context())
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
	}
	var size string
	var width, profile, diameter float64
	if err := pool.QueryRow(t.Context(), `SELECT size,width_value,aspect_ratio,diameter FROM catalog.tyre_specs WHERE product_id=$1`, tyre).Scan(&size, &width, &profile, &diameter); err != nil {
		t.Fatal(err)
	}
	if size != "210/90R17" || width != 210 || profile != 90 || diameter != 17 {
		t.Fatalf("wrong generated values: %s %v %v %v", size, width, profile, diameter)
	}
	for _, size := range []string{"", "322"} {
		if _, err := pool.Exec(t.Context(), `UPDATE catalog.tyre_specs SET size=$1 WHERE product_id=$2`, size, tyre); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(t.Context(), `UPDATE catalog.wheel_specs SET diameter_inches=NULL WHERE product_id=$1`, wheel); err != nil {
		t.Fatal(err)
	}
}
