//go:build integration

package httpapi

import (
	"strings"
	"testing"
	"tireshop/internal/catalogdata"
	"tireshop/internal/catalogsource"
	domain "tireshop/internal/requests"
	"tireshop/internal/requests/postgres"
)

func TestCatalogImportRetainsSourcesAndNeverRestoresDispatchedStock(t *testing.T) {
	catalog, orders := apiPools(t)
	sku := "TEST-BUNDLE"
	id := catalogsource.ProductID(sku)
	batch := catalogsource.Batch{Hash: strings.Repeat("a", 64), Workbooks: []catalogsource.Workbook{{Source: "synthetic.xlsx", SHA256: strings.Repeat("b", 64), Sheets: []catalogsource.Sheet{{Name: "Source", Rows: []catalogsource.Row{{Number: 2, Cells: []catalogsource.Cell{{Ref: "A2", Value: sku, Formula: `"TEST-BUNDLE"`}}}}}}}}, Products: []catalogsource.Product{{ID: id, Name: "Synthetic two-wheel set", Price: 25000, Stock: 1, Available: true, Details: catalogdata.Details{SKU: sku, Kind: "wheel", UnitsPerSale: 2, PCD: "10x335"}, SourceFile: "synthetic.xlsx", Sheet: "Source", Row: catalogsource.Row{Number: 2, Cells: []catalogsource.Cell{{Ref: "A2", Value: sku}}}}}}
	media := []catalogsource.Media{{ProductID: id, SourcePath: "synthetic.jpg", SHA256: strings.Repeat("c", 64), WebPPath: "/media/products/" + id + "/test.webp", Width: 1280, Height: 1280}}
	changed, err := catalogsource.Publish(t.Context(), catalog, batch, media)
	if err != nil || !changed {
		t.Fatalf("publish: %v %v", changed, err)
	}
	var count int
	if err = catalog.QueryRow(t.Context(), `SELECT count(*) FROM catalog.source_rows WHERE product_id=$1 AND raw_cells->>'number'='2'`, id).Scan(&count); err != nil || count != 1 {
		t.Fatal("source row not retained")
	}
	if err = catalog.QueryRow(t.Context(), `SELECT count(*) FROM catalog.product_images WHERE product_id=$1`, id).Scan(&count); err != nil || count != 1 {
		t.Fatal("image not retained")
	}
	store := postgres.New(orders)
	result, err := store.Create(t.Context(), "00000000-0000-4000-8000-000000000088", postgres.Submission{CustomerName: "Synthetic Set Buyer", Phone: "+79000000000", Items: []domain.RequestedItem{{ProductID: id, Quantity: 1, ExpectedUnitPriceRub: 25000}}})
	if err != nil || result.Code != "created" {
		t.Fatalf("create set request %v %s", err, result.Code)
	}
	after, err := store.AdminEdit(t.Context(), result.RequestID, "test", postgres.AdminEdit{Version: 1, Status: domain.Delivery})
	if err != nil {
		t.Fatal(err)
	}
	if after.TotalRub != 25000 || after.Items[0].UnitLabel != "комплект (2 шт.)" {
		t.Fatal("sale unit snapshot lost")
	}
	changed, err = catalogsource.Publish(t.Context(), catalog, batch, media)
	if err != nil || changed {
		t.Fatal("repeat import must be a no-op")
	}
	var stock int64
	if err = catalog.QueryRow(t.Context(), `SELECT stock FROM catalog.products WHERE product_id=$1`, id).Scan(&stock); err != nil || stock != 0 {
		t.Fatal("re-import restored stock")
	}
	// A different source containing the same SKU must fail as a whole.
	batch.Hash = strings.Repeat("d", 64)
	if _, err = catalogsource.Publish(t.Context(), catalog, batch, media); err == nil {
		t.Fatal("conflicting source silently overwrote product")
	}
	if err = catalog.QueryRow(t.Context(), `SELECT count(*) FROM catalog.import_batches WHERE source_hash=$1`, batch.Hash).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed batch partially persisted")
	}
	if _, err = orders.Exec(t.Context(), `UPDATE catalog.source_rows SET row_number=99`); err == nil {
		t.Fatal("requests wrote catalog import sources")
	}
}
