//go:build integration

package httpapi

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestCatalogFiltering(t *testing.T) {
	catalog, _ := apiPools(t)
	db := adminDB(t)
	const prefix = "Filter fixture "
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM catalog.products WHERE name LIKE 'Filter fixture %'`)
	})
	for i := 0; i < 29; i++ {
		_, err := db.Exec(t.Context(), `WITH p AS (INSERT INTO catalog.products(name,sku,kind,brand,production_year,unit_price_rub,stock,is_available) VALUES($1,$2,'tyre','Filter brand',2022,$3,$4,true) RETURNING product_id) INSERT INTO catalog.tyre_specs(product_id,size,tread_percent) SELECT product_id,'295/80R22.5',75 FROM p`, fmt.Sprintf("%s%02d", prefix, i), fmt.Sprintf("FILTER-%02d", i), 100+i, i)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := db.Exec(t.Context(), `WITH p AS (INSERT INTO catalog.products(name,kind,brand,unit_price_rub,stock,is_available) VALUES('Filter fixture wheel','wheel','Filter wheel',100,2,true),('Filter fixture secret','wheel','Filter secret',100,0,false) RETURNING product_id,is_available) INSERT INTO catalog.wheel_specs(product_id,pcd,offset_mm) SELECT product_id,CASE WHEN is_available THEN '6x222.25' ELSE 'secret' END,CASE WHEN is_available THEN -118.5 ELSE 118.5 END FROM p`)
	if err != nil {
		t.Fatal(err)
	}
	session := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer session.Close()
	t.Setenv("ADMIN_SESSION_URL", session.URL)
	server := httptest.NewServer(Catalog(catalog))
	defer server.Close()
	get := func(path string, admin bool, status int) []byte {
		t.Helper()
		req, _ := http.NewRequest("GET", server.URL+path, nil)
		if admin {
			req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "test"})
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != status {
			t.Fatalf("%s: %d %s", path, res.StatusCode, body)
		}
		return body
	}
	list := func(query string, admin bool, total, want int) []productDTO {
		t.Helper()
		path := "/api/products?"
		if admin {
			path = "/api/admin/products?"
		}
		var data struct {
			Items []productDTO `json:"items"`
			Total int          `json:"total"`
		}
		if err := json.Unmarshal(get(path+query, admin, 200), &data); err != nil {
			t.Fatal(err)
		}
		if data.Total != total || len(data.Items) != want {
			t.Fatalf("%s: total %d items %d", query, data.Total, len(data.Items))
		}
		return data.Items
	}
	q := "q=" + url.QueryEscape(prefix)
	first := list(q+"&kind=tyre&sort=unit_price_rub&limit=24", false, 29, 24)
	second := list(q+"&kind=tyre&sort=unit_price_rub&limit=24&offset=24", false, 29, 5)
	if first[23].UnitPriceRub != 123 || second[0].UnitPriceRub != 124 {
		t.Fatal("unstable pagination")
	}
	list(q+"&kind=tyre&sort=unit_price_rub&offset=100", false, 29, 0)
	list(q+"&kind=tyre&brand=Filter+brand&size=295%2F80R22.5&production_year=2022&rim_diameter=22.5&tyre_width=295+mm&aspect_ratio=80&tread_min=70&tread_max=80&price_min=100&price_max=105&in_stock=true", false, 5, 5)
	list(q+"&kind=wheel&pcd=6x222.25&wheel_offset=-118.5", false, 1, 1)
	list(q+"&brand=missing", false, 0, 0)
	list("q=FILTER-28", false, 1, 1)
	list(q+"&visibility=hidden&stock_max=0", true, 1, 1)
	list(q+"&kind=wheel", true, 2, 2)
	get("/api/admin/products/filters", false, 401)
	get("/api/products?visibility=hidden", false, 400)
	get("/api/products?sort=name%3BDROP", false, 400)
	for _, admin := range []bool{false, true} {
		path := "/api/products/filters?kind=wheel"
		if admin {
			path = "/api/admin/products/filters?kind=wheel"
		}
		var data struct {
			Options map[string][]string `json:"options"`
		}
		json.Unmarshal(get(path, admin, 200), &data)
		found := false
		for _, brand := range data.Options["brand"] {
			if brand == "Filter secret" {
				found = true
			}
			if brand == "Filter brand" {
				t.Fatal("tyre facet leaked into wheels")
			}
		}
		if found != admin {
			t.Fatal("facet visibility mismatch")
		}
	}
}
