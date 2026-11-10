//go:build integration

package httpapi

import (
	"context"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestStorefrontSelection(t *testing.T) {
	pool, _ := apiPools(t)
	db := adminDB(t)
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM catalog.products WHERE brand='Selection fixture'`)
	})
	_, err := db.Exec(t.Context(), `WITH p AS (
 INSERT INTO catalog.products(name,kind,brand,stock,units_per_sale,unit_price_rub,is_available)
 VALUES ('Select A','tyre','Selection fixture',4,1,100,true),('Select B','tyre','Selection fixture',2,2,200,true),
 ('Select C','tyre','Selection fixture',0,1,300,true),('Select secret','tyre','Selection fixture',99,1,100,false),
 ('Select unknown','tyre','Selection fixture',3,1,100,true),('Select other','tyre','Selection fixture',2,1,100,true)
 RETURNING product_id,name)
 INSERT INTO catalog.tyre_specs(product_id,size) SELECT product_id,CASE name
 WHEN 'Select A' THEN '195/70R17.5' WHEN 'Select B' THEN '195/70 r17,50'
 WHEN 'Select C' THEN '205/60R17.5' WHEN 'Select secret' THEN '225/80R24.5'
 WHEN 'Select other' THEN '7.50R16' ELSE '322' END FROM p;
 WITH p AS (INSERT INTO catalog.products(name,kind,brand,stock,units_per_sale,unit_price_rub)
 VALUES ('Select wheel','wheel','Selection fixture',2,4,100) RETURNING product_id)
 INSERT INTO catalog.wheel_specs(product_id,diameter_inches,width_inches,pcd,offset_mm,center_bore_mm)
 SELECT product_id,17.5,6.75,'6x222.25',-10,164 FROM p;`)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(Catalog(pool))
	defer server.Close()
	get := func(path string, status int) []byte {
		t.Helper()
		res, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != status {
			t.Fatalf("%s: %d %s", path, res.StatusCode, body)
		}
		return body
	}
	selectOptions := func(query string) selectionResponse {
		t.Helper()
		var r selectionResponse
		if err := json.Unmarshal(get("/api/products/selection?brand=Selection+fixture&"+query, 200), &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	canonical := selectOptions("kind=tyre&diameter=17.5")
	if len(canonical.Diameters) != 2 || len(canonical.Radii) != 2 || len(canonical.Sizes) != 2 {
		t.Fatal(canonical)
	}
	r := selectOptions("kind=tyre&radius=17.5&tyre_size=195%2F70R17.5")
	if len(r.Radii) != 2 || r.Radii[0].Value != "16" || r.Radii[1].UnitCount != "8" || r.Radii[1].ProductCount != 3 || len(r.Sizes) != 2 || r.Sizes[0].Value != "195/70R17.5" || r.Sizes[0].UnitCount != "8" || r.Sizes[1].UnitCount != "0" {
		t.Fatalf("wrong aggregation: %+v", r)
	}
	if r := selectOptions("kind=tyre"); len(r.Sizes) != 0 {
		t.Fatal(r)
	}
	if r := selectOptions("kind=tyre&rim_diameter=17.5&size=missing&in_stock=true"); len(r.Sizes) != 1 || len(r.Radii) != 2 {
		t.Fatal(r)
	}
	if r := selectOptions("kind=tyre&radius=17.5&rim_diameter=16"); len(r.Sizes) != 0 || len(r.Radii) != 2 {
		t.Fatal(r)
	}
	if r := selectOptions("kind=tyre&radius=17.5&price_max=100"); len(r.Sizes) != 1 || r.Sizes[0].UnitCount != "4" {
		t.Fatal(r)
	}
	if r := selectOptions("kind=wheel&radius=17.5&width=999&pcd=6x222.25&wheel_offset=-10&center_bore=164"); len(r.Sizes) != 1 || r.Sizes[0].Label != "6.75×17.5" || r.Sizes[0].UnitCount != "8" {
		t.Fatal(r)
	}
	for query, want := range map[string]int{
		"kind=tyre&radius=17.5&tyre_size=195%2F70R17.5":   2,
		"kind=tyre&diameter=17.5&tyre_size=195%2F70R17.5": 2,
		"kind=tyre&diameter=17.5&radius=16":               0,
		"kind=tyre&size=195%2F70R17.5":                    1,
		"kind=tyre&radius=16&tyre_size=195%2F70R17.5":     0,
		"kind=wheel&radius=17.5&width=6.75":               1,
	} {
		var data struct {
			Total int `json:"total"`
		}
		json.Unmarshal(get("/api/products?brand=Selection+fixture&"+query, 200), &data)
		if data.Total != want {
			t.Fatalf("%s: %d", query, data.Total)
		}
	}
	for _, q := range []string{"", "kind=tyre&limit=1", "kind=wheel&tyre_size=x", "kind=tyre&radius=NaN", "kind=tyre&radius=0", "kind=tyre&radius=17&radius=18", "kind=tyre&visibility=hidden", "kind=tyre&unknown=x"} {
		get("/api/products/selection?"+q, 400)
	}
	// Changes through the same underlying sources used by the old editor need no cache rebuild.
	_, err = db.Exec(t.Context(), `UPDATE catalog.tyre_specs SET size='205/60R19.5' WHERE product_id IN (SELECT product_id FROM catalog.products WHERE name='Select A'); UPDATE catalog.products SET stock=9007199254740991,units_per_sale=100 WHERE name='Select wheel'`)
	if err != nil {
		t.Fatal(err)
	}
	r = selectOptions("kind=tyre&radius=19.5")
	if len(r.Sizes) != 1 || r.Sizes[0].Value != "205/60R19.5" {
		t.Fatal(r)
	}
	r = selectOptions("kind=wheel&radius=17.5")
	if r.Sizes[0].UnitCount != "900719925474099100" {
		t.Fatal(r)
	}
	// Runtime permissions and immutable normalized keys.
	var key string
	if err := pool.QueryRow(t.Context(), `SELECT catalog.tyre_size_key_v1($1)`, " 7,50 r16 ").Scan(&key); err != nil || key != "7.50R16" {
		t.Fatalf("%s %v", key, err)
	}
	for raw, want := range map[string]string{"165R13": "165R13", "11R22.5": "11.00R22.5", "195/70R17.50": "195/70R17.5"} {
		if err := pool.QueryRow(t.Context(), `SELECT catalog.tyre_size_key_v1($1)`, raw).Scan(&key); err != nil || key != want {
			t.Fatalf("%s %s %v", raw, key, err)
		}
	}
	// Literal search never executes SQL from parameters.
	get("/api/products/selection?kind=tyre&q="+url.QueryEscape("'; DROP TABLE catalog.products;--"), 200)
}
