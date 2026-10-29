//go:build integration

package httpapi

import (
	"testing"
	"tireshop/internal/catalogdata"
)

func TestTypedSpecificationsAreAtomicAndRestricted(t *testing.T) {
	catalog, orders := apiPools(t)
	id := newTestProduct(t, catalog, 7)
	save := func(d catalogdata.Details, commit bool) error {
		tx, err := catalog.Begin(t.Context())
		if err != nil {
			return err
		}
		defer tx.Rollback(t.Context())
		if err = catalogdata.Save(t.Context(), tx, id, d); err != nil {
			return err
		}
		if commit {
			return tx.Commit(t.Context())
		}
		return nil
	}
	d := catalogdata.Details{SKU: "TYPED-TEST", Kind: "tyre", Size: "7.50R16", UnitsPerSale: 1}
	if err := save(d, true); err != nil {
		t.Fatal(err)
	}
	var width float64
	var unit string
	var profile *float64
	if err := catalog.QueryRow(t.Context(), `SELECT width_value,width_unit,aspect_ratio FROM catalog.tyre_specs WHERE product_id=$1`, id).Scan(&width, &unit, &profile); err != nil || width != 7.5 || unit != "in" || profile != nil {
		t.Fatalf("inch width: %v %s %v %v", width, unit, profile, err)
	}
	d.Size = "165R13"
	if err := save(d, true); err != nil {
		t.Fatal(err)
	}
	if err := catalog.QueryRow(t.Context(), `SELECT width_value,width_unit,aspect_ratio FROM catalog.tyre_specs WHERE product_id=$1`, id).Scan(&width, &unit, &profile); err != nil || width != 165 || unit != "mm" || profile != nil {
		t.Fatalf("metric without profile: %v %s %v %v", width, unit, profile, err)
	}
	if _, err := catalog.Exec(t.Context(), `INSERT INTO catalog.wheel_specs(product_id) VALUES($1)`, id); err == nil {
		t.Fatal("wrong subtype accepted")
	}
	offset := -118.5
	wheel := catalogdata.Details{SKU: "TYPED-TEST", Kind: "wheel", WheelOffset: &offset, PCD: "6x222.25", UnitsPerSale: 1}
	if err := save(wheel, false); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := catalog.QueryRow(t.Context(), `SELECT count(*) FROM catalog.tyre_specs WHERE product_id=$1`, id).Scan(&count); err != nil || count != 1 {
		t.Fatal("rollback lost tyre specs")
	}
	if err := save(wheel, true); err != nil {
		t.Fatal(err)
	}
	if err := catalog.QueryRow(t.Context(), `SELECT count(*) FROM catalog.tyre_specs WHERE product_id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatal("old subtype retained")
	}
	if err := catalog.QueryRow(t.Context(), `SELECT offset_mm FROM catalog.wheel_specs WHERE product_id=$1`, id).Scan(&width); err != nil || width != offset {
		t.Fatal("wheel offset lost")
	}
	for _, table := range []string{"catalog.tyre_specs", "catalog.wheel_specs", "catalog.product_details", "catalog.specs_migration_archive"} {
		if _, err := orders.Exec(t.Context(), "SELECT * FROM "+table); err == nil {
			t.Fatalf("requests accessed %s", table)
		}
	}
	if _, err := catalog.Exec(t.Context(), `DELETE FROM catalog.specs_migration_archive`); err == nil {
		t.Fatal("runtime modified migration archive")
	}
}
