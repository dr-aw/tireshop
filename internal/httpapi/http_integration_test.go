//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests use real SQL permissions and migrations in the disposable DB
// created by test-db.sh. HTTP assertions alone cannot prove atomic persistence.
func apiPools(t *testing.T) (*pgxpool.Pool, *pgxpool.Pool) {
	t.Helper()
	db := os.Getenv("TIRESHOP_TEST_DB")
	if !regexp.MustCompile(`^tireshop_test_[a-f0-9]{16}$`).MatchString(db) || os.Getenv("PGDATABASE") != db || os.Getenv("PGHOST") != "postgres" || os.Getenv("PGPORT") != "5432" {
		t.Fatal("Use mise run test:integration with its temporary local DB")
	}
	var pools []*pgxpool.Pool
	for _, role := range []string{"_catalog", "_requests"} {
		cfg, err := pgxpool.ParseConfig("")
		if err != nil {
			t.Fatal("test DB config")
		}
		cfg.ConnConfig.RuntimeParams["role"] = db + role
		cfg.MaxConns = 8
		pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
		if err != nil {
			t.Fatal("test pool")
		}
		t.Cleanup(pool.Close)
		if err := pool.Ping(t.Context()); err != nil {
			t.Fatal("test DB unavailable")
		}
		pools = append(pools, pool)
	}
	return pools[0], pools[1]
}

func TestHTTPCheckout(t *testing.T) {
	catalog, requests := apiPools(t)
	const product = "00000000-0000-4000-8000-000000000001"
	if _, err := catalog.Exec(t.Context(), `INSERT INTO catalog.products(product_id,name,unit_price_rub) VALUES ($1,'HTTP synthetic tyre',1000)`, product); err != nil {
		t.Fatal(err)
	}
	cs := httptest.NewServer(Catalog(catalog))
	defer cs.Close()
	rs := httptest.NewServer(Requests(requests, []string{"http://localhost:5173"}))
	defer rs.Close()
	payload := fmt.Sprintf(`{"customer_name":"HTTP Buyer","phone":"+79000000000","comment":"Test","items":[{"product_id":%q,"quantity":2,"expected_unit_price_rub":1000}]}`, product)
	call := func(method, url, key, body, contentType, origin string, status int) map[string]any {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), method, url, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Origin", origin)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != status {
			t.Fatalf("%s %s: status %d, want %d: %s", method, url, resp.StatusCode, status, raw)
		}
		var result map[string]any
		if strings.Contains(resp.Header.Get("Content-Type"), "application/json") {
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
		}
		return result
	}
	call("GET", cs.URL+"/api/products/"+product, "", "", "", "", 200)
	call("GET", cs.URL+"/api/products?limit=101", "", "", "", "", 400)
	call("GET", rs.URL+"/api/requests", "", "", "", "", 405)
	const key = "00000000-0000-4000-8000-000000000002"
	first := call("POST", rs.URL+"/api/requests", key, payload, "application/json", "http://localhost:5173", 201)
	replay := call("POST", rs.URL+"/api/requests", key, payload, "application/json", "", 201)
	if first["request_id"] == nil || first["request_id"] != replay["request_id"] {
		t.Fatal("retry did not return the original request")
	}
	conflict := call("POST", rs.URL+"/api/requests", key, strings.Replace(payload, "HTTP Buyer", "Different Buyer", 1), "application/json", "", 409)
	if conflict["error"].(map[string]any)["code"] != "idempotency_conflict" {
		t.Fatal("wrong conflict")
	}
	for _, tc := range []struct {
		name, body, ct, origin string
		status                 int
	}{
		{"unknown field", `{"unexpected":true}`, "application/json", "", 400},
		{"duplicate field", `{"phone":"a","phone":"b"}`, "application/json", "", 400},
		{"missing price", strings.Replace(payload, `,"expected_unit_price_rub":1000`, "", 1), "application/json", "", 422},
		{"body limit", strings.Repeat(" ", 65537), "application/json", "", 413},
		{"media type", payload, "text/plain", "", 415},
		{"origin", payload, "application/json", "https://untrusted.invalid", 403},
	} {
		t.Run(tc.name, func(t *testing.T) { call("POST", rs.URL+"/api/requests", key, tc.body, tc.ct, tc.origin, tc.status) })
	}
	if _, err := catalog.Exec(t.Context(), `UPDATE catalog.products SET unit_price_rub=1200 WHERE product_id=$1`, product); err != nil {
		t.Fatal(err)
	}
	const quoteKey = "00000000-0000-4000-8000-000000000003"
	quote := call("POST", rs.URL+"/api/requests", quoteKey, payload, "application/json", "", 409)
	if quote["total_rub"] != float64(2400) {
		t.Fatal("wrong new quote")
	}
	confirmed := strings.Replace(payload, `"expected_unit_price_rub":1000`, `"expected_unit_price_rub":1200`, 1)
	call("POST", rs.URL+"/api/requests", "00000000-0000-4000-8000-000000000004", confirmed, "application/json", "", 201)
	var n int
	if err := requests.QueryRow(t.Context(), `SELECT count(*) FROM requests.requests WHERE customer_name='HTTP Buyer'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("want exactly initial+confirmed requests: n=%d err=%v", n, err)
	}
}

func TestBrowserLostResponse(t *testing.T) {
	if os.Getenv("TIRESHOP_BROWSER_TEST") != "1" {
		t.Skip("Run mise run test:browser for Chromium E2E")
	}
	catalog, requests := apiPools(t)
	const id = "00000000-0000-4000-8000-000000000000"
	if _, err := catalog.Exec(t.Context(), `INSERT INTO catalog.products(product_id,name,unit_price_rub) VALUES ($1,'Browser synthetic tyre',18000)`, id); err != nil {
		t.Fatal(err)
	}
	const origin = "http://127.0.0.1:5174"
	for i := 0; i < 30; i++ {
		_, err := catalog.Exec(t.Context(), `INSERT INTO catalog.products(product_id,name,sku,kind,brand,size,diameter,tread_percent,unit_price_rub,stock) VALUES($1,$2,$3,'tyre','Browser Filter Brand','315/70R22.5',22.5,80,$4,4)`, fmt.Sprintf("10000000-0000-4000-8000-%012d", i), fmt.Sprintf("Browser filter tyre %02d", i), fmt.Sprintf("BROWSER-FILTER-%02d", i), 1000+i)
		if err != nil {
			t.Fatal(err)
		}
	}
	const bundleID = "00000000-0000-4000-8000-000000000091"
	if _, err := catalog.Exec(t.Context(), `INSERT INTO catalog.products(product_id,name,unit_price_rub,stock,sku,kind,units_per_sale,description) VALUES($1,'Browser wheel bundle',25000,1,'BROWSER-SET','wheel',2,'Synthetic gallery fixture')`, bundleID); err != nil {
		t.Fatal(err)
	}
	for i, hash := range []string{strings.Repeat("e", 64), strings.Repeat("f", 64)} {
		if _, err := catalog.Exec(t.Context(), `INSERT INTO catalog.product_images(product_id,position,source_path,source_sha256,webp_path,width,height) VALUES($1,$2,'synthetic.jpg',$3,$4,1280,1280)`, bundleID, i, hash, fmt.Sprintf("/media/test-%d-1280-v1.webp", i)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ADMIN_LOCAL_HTTP", "1")
	provisionTestAdmin(t, adminDB(t), "browser-admin")
	rs := httptest.NewServer(Requests(requests, []string{origin}))
	defer rs.Close()
	t.Setenv("ADMIN_SESSION_URL", rs.URL+"/api/admin/session")
	t.Setenv("APP_ORIGINS", origin)
	cs := httptest.NewServer(Catalog(catalog))
	defer cs.Close()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	// Child processes do not need DB credentials: they only talk HTTP.
	env := []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "CATALOG_PROXY_TARGET=" + cs.URL, "REQUESTS_PROXY_TARGET=" + rs.URL}
	// Allow a user-local Chromium library/font installation in an older dev
	// container; never pass the PostgreSQL environment to browser processes.
	for _, key := range []string{"LD_LIBRARY_PATH", "FONTCONFIG_FILE"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	vite := exec.CommandContext(ctx, "vite", "frontend", "--host", "127.0.0.1", "--port", "5174", "--strictPort")
	vite.Dir = root
	vite.Env = env
	vite.Stdout = os.Stderr
	vite.Stderr = os.Stderr
	if err := vite.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = vite.Process.Kill(); _ = vite.Wait() }()
	ready := false
	client := &http.Client{Timeout: time.Second}
	for range 100 {
		resp, err := client.Get(origin)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("Vite startup timeout")
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatal("Vite failed to start")
	}
	where := exec.CommandContext(ctx, "mise", "where", "npm:@playwright/test")
	location, err := where.Output()
	if err != nil {
		t.Fatal("locating pinned Playwright", err)
	}
	runner := exec.CommandContext(ctx, "playwright", "test", "--config", "frontend/e2e/playwright.config.cjs")
	runner.Dir = root
	runner.Env = append(env, "BASE_URL="+origin, "PLAYWRIGHT_TEST_MODULE="+filepath.Join(strings.TrimSpace(string(location)), "node_modules/@playwright/test"))
	var output bytes.Buffer
	runner.Stdout = &output
	runner.Stderr = &output
	if err := runner.Run(); err != nil {
		t.Fatalf("browser test: %v\n%s", err, output.String())
	}
	t.Log(output.String())
	var unit string
	var bundleTotal int64
	if err := requests.QueryRow(t.Context(), `SELECT i.unit_label,r.total_rub FROM requests.request_items i JOIN requests.requests r USING(request_id) WHERE r.customer_name='Browser Bundle Buyer'`).Scan(&unit, &bundleTotal); err != nil || unit != "комплект (2 шт.)" || bundleTotal != 25000 {
		t.Fatalf("bundle snapshot: %q %d %v", unit, bundleTotal, err)
	}
	var n int
	if err := requests.QueryRow(t.Context(), `SELECT count(*) FROM requests.requests WHERE customer_name='Browser Buyer'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("browser retry duplicated/lost submission: n=%d err=%v", n, err)
	}
}
