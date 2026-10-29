//go:build integration

package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"tireshop/internal/adminauth"
	domain "tireshop/internal/requests"
	"tireshop/internal/requests/postgres"
)

func adminDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(t.Context(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
func provisionTestAdmin(t *testing.T, pool *pgxpool.Pool, name string) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `INSERT INTO requests.admin_users(username,password_hash) VALUES($1,$2)`, name, adminauth.Hash("synthetic-test-password"))
	if err != nil {
		t.Fatal(err)
	}
}
func newTestProduct(t *testing.T, pool *pgxpool.Pool, stock int64) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(t.Context(), `INSERT INTO catalog.products(name,unit_price_rub,stock) VALUES('Admin synthetic tyre',100,$1) RETURNING product_id::text`, stock).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func newTestRequest(t *testing.T, pool *pgxpool.Pool, ids []string, qty int64) string {
	t.Helper()
	var key string
	if err := pool.QueryRow(t.Context(), `SELECT gen_random_uuid()::text`).Scan(&key); err != nil {
		t.Fatal(err)
	}
	input := postgres.Submission{CustomerName: "Admin synthetic buyer", Phone: "+79000000000", Comment: "<script>alert(1)</script>"}
	for _, id := range ids {
		input.Items = append(input.Items, domain.RequestedItem{ProductID: id, Quantity: qty, ExpectedUnitPriceRub: 100})
	}
	result, err := postgres.New(pool).Create(t.Context(), key, input)
	if err != nil || result.Code != "created" {
		t.Fatalf("create: %v %s", err, result.Code)
	}
	return result.RequestID
}
func TestAdministratorHTTP(t *testing.T) {
	catalog, requests := apiPools(t)
	admin := adminDB(t)
	provisionTestAdmin(t, admin, "operator")
	t.Setenv("ADMIN_LOCAL_HTTP", "1")
	const origin = "http://localhost:5173"
	rs := httptest.NewServer(Requests(requests, []string{origin}))
	defer rs.Close()
	t.Setenv("ADMIN_SESSION_URL", rs.URL+"/api/admin/session")
	cs := httptest.NewServer(Catalog(catalog))
	defer cs.Close()
	var cookie *http.Cookie
	call := func(method, url string, body any, csrf bool, status int) []byte {
		t.Helper()
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, url, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		if csrf {
			req.Header.Set("Origin", origin)
			req.Header.Set("X-Admin-Request", "1")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != status {
			t.Fatalf("%s %s: %d want %d: %s", method, url, resp.StatusCode, status, out)
		}
		if method == "POST" && strings.HasSuffix(url, "/login") && status == 200 {
			cookie = resp.Cookies()[0]
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 259200 {
				t.Fatal("cookie policy")
			}
		}
		return out
	}
	call("GET", rs.URL+"/api/admin/requests", nil, false, 401)
	call("GET", cs.URL+"/api/admin/products", nil, false, 401)
	credentials := map[string]string{"username": "operator", "password": "synthetic-test-password"}
	call("POST", rs.URL+"/api/admin/login", credentials, false, 403)
	call("POST", rs.URL+"/api/admin/login", map[string]string{"username": "operator", "password": "wrong"}, true, 401)
	call("POST", rs.URL+"/api/admin/login", credentials, true, 200)
	call("GET", rs.URL+"/api/admin/session", nil, false, 200)
	call("GET", rs.URL+"/api/admin/requests", nil, false, 200)
	call("GET", cs.URL+"/api/admin/products", nil, false, 200)
	id := newTestProduct(t, catalog, 8)
	order := newTestRequest(t, requests, []string{id}, 2)
	var p adminProduct
	price, stock, available := int64(150), int64(9), true
	p = adminProduct{ID: id, Name: "Renamed tyre", UnitPriceRub: &price, Stock: &stock, Available: &available, Version: 1}
	call("PUT", cs.URL+"/api/admin/products/"+id, p, false, 403)
	call("PUT", cs.URL+"/api/admin/products/"+id, p, true, 200)
	call("PUT", cs.URL+"/api/admin/products/"+id, p, true, 409)
	call("GET", cs.URL+"/api/admin/products?sort=not_a_column", nil, false, 400)
	for _, direction := range []string{"asc", "desc"} {
		var result struct {
			Items []adminProduct `json:"items"`
		}
		if err := json.Unmarshal(call("GET", cs.URL+"/api/admin/products?sort=unit_price_rub&direction="+direction, nil, false, 200), &result); err != nil {
			t.Fatal(err)
		}
		for i := 1; i < len(result.Items); i++ {
			a, b := *result.Items[i-1].UnitPriceRub, *result.Items[i].UnitPriceRub
			if direction == "asc" && a > b || direction == "desc" && a < b {
				t.Fatal("catalog sort order incorrect")
			}
		}
	}
	var edit postgres.AdminEdit
	if err := json.Unmarshal([]byte(fmt.Sprintf(`{"version":1,"status":"delivery","comment":"Sent","items":[{"product_id":%q,"quantity":3}]}`, id)), &edit); err != nil {
		t.Fatal(err)
	}
	call("PUT", rs.URL+"/api/admin/requests/"+order, edit, false, 403)
	call("PUT", rs.URL+"/api/admin/requests/"+order, edit, true, 200)
	call("PUT", rs.URL+"/api/admin/requests/"+order, edit, true, 409)
	var detail postgres.AdminDetail
	if err := json.Unmarshal(call("GET", rs.URL+"/api/admin/requests/"+order, nil, false, 200), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Request.TotalRub != 300 || len(detail.History) != 1 || detail.History[0].Before.TotalRub != 200 || detail.History[0].Actor != "operator" {
		t.Fatalf("snapshot/history: %+v", detail)
	}
	var remaining int64
	if err := catalog.QueryRow(t.Context(), `SELECT stock FROM catalog.products WHERE product_id=$1`, id).Scan(&remaining); err != nil || remaining != 6 {
		t.Fatalf("stock %d %v", remaining, err)
	}
	edit.Version = 2
	edit.Status = domain.Delivered
	call("PUT", rs.URL+"/api/admin/requests/"+order, edit, true, 422) // frozen composition
	edit.Items = nil
	call("PUT", rs.URL+"/api/admin/requests/"+order, edit, true, 200)
	edit.Version = 3
	edit.Status = domain.Cancelled
	edit.Comment = " "
	call("PUT", rs.URL+"/api/admin/requests/"+order, edit, true, 422)
	edit.Comment = "Return requires inspection"
	call("PUT", rs.URL+"/api/admin/requests/"+order, edit, true, 200)
	if err := catalog.QueryRow(t.Context(), `SELECT stock FROM catalog.products WHERE product_id=$1`, id).Scan(&remaining); err != nil || remaining != 6 {
		t.Fatal("cancellation changed stock")
	}
	// Catalog does not get access to authentication data or request writes.
	if _, err := catalog.Exec(t.Context(), `SELECT * FROM requests.admin_users`); err == nil {
		t.Fatal("catalog can read users")
	}
	if _, err := requests.Exec(t.Context(), `UPDATE catalog.products SET stock=99`); err == nil {
		t.Fatal("requests can write catalog")
	}
	digest := sha256.Sum256([]byte(cookie.Value))
	if _, err := admin.Exec(t.Context(), `UPDATE requests.admin_sessions SET expires_at=now()-interval '1 second' WHERE token_hash=$1`, digest[:]); err != nil {
		t.Fatal(err)
	}
	call("GET", rs.URL+"/api/admin/requests", nil, false, 401)
	call("GET", cs.URL+"/api/admin/products", nil, false, 401)
	call("POST", rs.URL+"/api/admin/login", credentials, true, 200)
	if _, err := admin.Exec(t.Context(), `UPDATE requests.admin_users SET active=false WHERE username='operator'`); err != nil {
		t.Fatal(err)
	}
	call("GET", rs.URL+"/api/admin/session", nil, false, 401)
	if _, err := admin.Exec(t.Context(), `UPDATE requests.admin_users SET active=true WHERE username='operator'`); err != nil {
		t.Fatal(err)
	}
	call("POST", rs.URL+"/api/admin/logout", map[string]bool{}, true, 200)
	call("GET", rs.URL+"/api/admin/session", nil, false, 401)
	call("POST", rs.URL+"/api/admin/login", credentials, true, 200)
	requests.Close()
	call("GET", rs.URL+"/api/admin/requests", nil, false, 503)
	call("GET", cs.URL+"/api/admin/products", nil, false, 503)
}

func TestDispatchAtomicAndConcurrent(t *testing.T) {
	catalog, pool := apiPools(t)
	store := postgres.New(pool)
	first := newTestProduct(t, catalog, 10)
	second := newTestProduct(t, catalog, 1)
	id := newTestRequest(t, pool, []string{first, second}, 2)
	edit := postgres.AdminEdit{Version: 1, Status: domain.Delivery, Comment: "dispatch"}
	if _, err := store.AdminEdit(t.Context(), id, "tester", edit); !errors.Is(err, postgres.ErrStock) {
		t.Fatalf("want shortage: %v", err)
	}
	detail, err := store.AdminGet(t.Context(), id)
	if err != nil || detail.Request.Status != domain.Created || detail.Request.Version != 1 || len(detail.History) != 0 {
		t.Fatal("failed dispatch changed request")
	}
	var stock int64
	if err := catalog.QueryRow(t.Context(), `SELECT stock FROM catalog.products WHERE product_id=$1`, first).Scan(&stock); err != nil || stock != 10 {
		t.Fatal("partial stock deduction")
	}
	if _, err := catalog.Exec(t.Context(), `UPDATE catalog.products SET stock=5 WHERE product_id=$1`, second); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for range 12 {
		wg.Go(func() { _, err := store.AdminEdit(context.Background(), id, "tester", edit); results <- err })
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, postgres.ErrVersion) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("duplicate dispatch: %d", success)
	}
	if err := catalog.QueryRow(t.Context(), `SELECT stock FROM catalog.products WHERE product_id=$1`, first).Scan(&stock); err != nil || stock != 8 {
		t.Fatal("duplicate stock deduction")
	}
	// Different requests compete for the same stock, without going negative.
	shared := newTestProduct(t, catalog, 3)
	a := newTestRequest(t, pool, []string{shared}, 2)
	b := newTestRequest(t, pool, []string{shared}, 2)
	results = make(chan error, 2)
	for _, order := range []string{a, b} {
		wg.Go(func() { _, err := store.AdminEdit(context.Background(), order, "tester", edit); results <- err })
	}
	wg.Wait()
	close(results)
	success = 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, postgres.ErrStock) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("overselling")
	}
	if err := catalog.QueryRow(t.Context(), `SELECT stock FROM catalog.products WHERE product_id=$1`, shared).Scan(&stock); err != nil || stock != 1 {
		t.Fatal("invalid final stock")
	}
}

func TestSecureCookieDefault(t *testing.T) {
	t.Setenv("ADMIN_LOCAL_HTTP", "")
	if !cookieFor("x", 1).Secure {
		t.Fatal("insecure default cookie")
	}
}

func TestAdminPasswordResetCLI(t *testing.T) {
	_, _ = apiPools(t) // Validate that this is the isolated local test database.
	admin := adminDB(t)
	provisionTestAdmin(t, admin, "reset-user")
	digest := sha256.Sum256([]byte("synthetic-session-to-revoke"))
	if _, err := admin.Exec(t.Context(), `INSERT INTO requests.admin_sessions(token_hash,username,expires_at) VALUES($1,'reset-user',now()+interval '1 hour')`, digest[:]); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(file, []byte("replacement-synthetic-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "go", "run", "../../cmd/admin-user", "reset-user", file)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reset CLI: %v %s", err, out)
	}
	var hash string
	var sessions int
	if err := admin.QueryRow(t.Context(), `SELECT password_hash FROM requests.admin_users WHERE username='reset-user'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !adminauth.Verify(hash, "replacement-synthetic-password") || adminauth.Verify(hash, "synthetic-test-password") {
		t.Fatal("password reset failed")
	}
	if err := admin.QueryRow(t.Context(), `SELECT count(*) FROM requests.admin_sessions WHERE username='reset-user'`).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatal("sessions survived password reset")
	}
}

func TestDispatchRollbackAfterCatalogWrite(t *testing.T) {
	catalog, pool := apiPools(t)
	admin := adminDB(t)
	store := postgres.New(pool)
	id := newTestProduct(t, catalog, 7)
	order := newTestRequest(t, pool, []string{id}, 2)
	_, err := admin.Exec(t.Context(), `CREATE FUNCTION requests.fail_test_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.actor='fail-audit' THEN RAISE EXCEPTION 'synthetic audit failure'; END IF; RETURN NEW; END $$;
	CREATE TRIGGER fail_test_audit BEFORE INSERT ON requests.request_history FOR EACH ROW EXECUTE FUNCTION requests.fail_test_audit()`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `DROP TRIGGER fail_test_audit ON requests.request_history; DROP FUNCTION requests.fail_test_audit()`)
	})
	_, err = store.AdminEdit(t.Context(), order, "fail-audit", postgres.AdminEdit{Version: 1, Status: domain.Delivery})
	if err == nil {
		t.Fatal("expected audit failure")
	}
	var stock int64
	if err := catalog.QueryRow(t.Context(), `SELECT stock FROM catalog.products WHERE product_id=$1`, id).Scan(&stock); err != nil || stock != 7 {
		t.Fatal("stock survived rolled-back request update")
	}
	detail, err := store.AdminGet(t.Context(), order)
	if err != nil || detail.Request.Version != 1 || detail.Request.Status != domain.Created || len(detail.History) != 0 {
		t.Fatal("request survived rolled-back audit")
	}
}
