//go:build integration

package httpapi

import (
	"context"
	"encoding/json/v2"
	"net/http/httptest"
	"testing"
)

func TestPublicPhotoOrderBeforePagination(t *testing.T) {
	pool, _ := apiPools(t)
	db := adminDB(t)
	const plain = "11111111-0000-4000-8000-000000000001"
	const photo = "eeeeeeee-0000-4000-8000-000000000003"
	_, err := db.Exec(t.Context(), `INSERT INTO catalog.products(product_id,name,unit_price_rub) VALUES ($1,'Photo order fixture plain',1),($2,'Photo order fixture image',2)`, plain, photo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM catalog.product_images WHERE product_id=$1`, photo)
		_, _ = db.Exec(context.Background(), `DELETE FROM catalog.products WHERE product_id IN ($1,$2)`, plain, photo)
	})
	_, err = db.Exec(t.Context(), `INSERT INTO catalog.product_images(product_id,position,source_path,source_sha256,webp_path,width,height) VALUES ($1,0,'synthetic','synthetic','/media/synthetic.webp',100,100)`, photo)
	if err != nil {
		t.Fatal(err)
	}
	h := Catalog(pool)
	for _, v := range []struct{ query, want string }{{"offset=0", photo}, {"offset=1", plain}, {"sort=unit_price_rub&direction=asc", plain}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/products?q=Photo+order+fixture&limit=1&"+v.query, nil))
		var result struct {
			Items []productDTO `json:"items"`
			Total int          `json:"total"`
		}
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Total != 2 || len(result.Items) != 1 || result.Items[0].ID != v.want {
			t.Fatalf("%s: %+v", v.query, result)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/products/snapshot", nil))
	var snap struct {
		Items []productDTO `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	plainIndex, photoIndex := -1, -1
	for i, p := range snap.Items {
		if p.ID == plain {
			plainIndex = i
		}
		if p.ID == photo {
			photoIndex = i
		}
	}
	if photoIndex < 0 || plainIndex < 0 || photoIndex >= plainIndex {
		t.Fatal("snapshot order differs from public list")
	}
}
