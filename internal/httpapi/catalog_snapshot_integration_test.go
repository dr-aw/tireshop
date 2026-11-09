//go:build integration

package httpapi

import (
	"context"
	"encoding/json/v2"
	"net/http/httptest"
	"testing"
)

func TestPublicSnapshot(t *testing.T) {
	pool, _ := apiPools(t)
	db := adminDB(t)
	_, err := db.Exec(t.Context(), `INSERT INTO catalog.products(product_id,name,unit_price_rub,stock,is_available,location,review_notes) VALUES ('eeeeeeee-0000-4000-8000-000000000001','Snapshot visible',123,0,true,'private shelf','private note'),('eeeeeeee-0000-4000-8000-000000000002','Snapshot hidden',123,2,false,'private shelf','private note')`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM catalog.products WHERE product_id IN ('eeeeeeee-0000-4000-8000-000000000001','eeeeeeee-0000-4000-8000-000000000002')`)
	})
	handler := Catalog(pool)
	get := func() (string, []productDTO) {
		t.Helper()
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/products/snapshot", nil))
		if w.Code != 200 {
			t.Fatalf("snapshot status %d: %s", w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("snapshot must bypass cache")
		}
		var value struct {
			Version string       `json:"version"`
			Items   []productDTO `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		return value.Version, value.Items
	}
	first, items := get()
	found := false
	for _, p := range items {
		if p.ID == "eeeeeeee-0000-4000-8000-000000000002" {
			t.Fatal("hidden product exported")
		}
		if p.Details.Location != "" || p.Details.ReviewNotes != "" {
			t.Fatal("private fields exported")
		}
		if p.ID == "eeeeeeee-0000-4000-8000-000000000001" {
			found = true
			if p.Stock != 0 {
				t.Fatal("stock")
			}
		}
	}
	if !found {
		t.Fatal("zero-stock public product omitted")
	}
	second, _ := get()
	if first != second {
		t.Fatal("unchanged data changed version")
	}
	if _, err = db.Exec(t.Context(), `UPDATE catalog.products SET unit_price_rub=124 WHERE product_id='eeeeeeee-0000-4000-8000-000000000001'`); err != nil {
		t.Fatal(err)
	}
	third, _ := get()
	if third == first {
		t.Fatal("price change did not change version")
	}
}
