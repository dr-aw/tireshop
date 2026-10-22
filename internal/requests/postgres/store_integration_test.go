//go:build integration

package postgres

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"tireshop/internal/requests"
)

var adminPool, catalogPool, requestsPool *pgxpool.Pool

func TestMain(m *testing.M) {
	db := os.Getenv("TIRESHOP_TEST_DB")
	if !regexp.MustCompile(`^tireshop_test_[a-f0-9]{16}$`).MatchString(db) ||
		os.Getenv("PGDATABASE") != db || os.Getenv("PGHOST") != "postgres" || os.Getenv("PGPORT") != "5432" {
		fmt.Fprintln(os.Stderr, "Use mise run test:integration; an isolated local test DB is required.")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pools := []**pgxpool.Pool{&adminPool, &catalogPool, &requestsPool}
	roles := []string{"", db + "_catalog", db + "_requests"}
	for i, role := range roles {
		config, err := pgxpool.ParseConfig("")
		if err != nil {
			fmt.Fprintln(os.Stderr, "Test database configuration failed.")
			os.Exit(1)
		}
		config.MaxConns = 8
		if role != "" {
			config.ConnConfig.RuntimeParams["role"] = role
		}
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err == nil {
			err = pool.Ping(ctx)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Test database connection/role failed.")
			os.Exit(1)
		}
		*pools[i] = pool
	}
	code := m.Run()
	requestsPool.Close()
	catalogPool.Close()
	adminPool.Close()
	os.Exit(code)
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func execSQL(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(testContext(t), sql, args...); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) Submission {
	t.Helper()
	ids := []string{newUUID(), newUUID()}
	for i, id := range ids {
		execSQL(t, catalogPool, `INSERT INTO catalog.products (product_id, name, unit_price_rub)
			VALUES ($1::uuid, $2, $3)`, id, fmt.Sprintf("Synthetic product %d", i), (i+1)*1000)
	}
	return Submission{CustomerName: "Synthetic Buyer", Phone: "+79000000000", Comment: "Test only",
		Items: []requests.RequestedItem{{ProductID: ids[0], Quantity: 2, ExpectedUnitPriceRub: 1000},
			{ProductID: ids[1], Quantity: 3, ExpectedUnitPriceRub: 2000}}}
}

func count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := adminPool.QueryRow(testContext(t), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateSnapshotAndReplay(t *testing.T) {
	ctx := testContext(t)
	input, key := fixture(t), newUUID()
	store := New(requestsPool)
	result, err := store.Create(ctx, key, input)
	if err != nil || result.Code != "created" || result.RequestID == "" {
		t.Fatalf("result=%+v, err=%v", result, err)
	}
	var total int64
	var status string
	if err := adminPool.QueryRow(ctx, `SELECT total_rub, status FROM requests.requests
		WHERE request_id=$1::uuid`, result.RequestID).Scan(&total, &status); err != nil {
		t.Fatal(err)
	}
	if total != 8000 || status != "created" || count(t, `SELECT count(*) FROM requests.request_items WHERE request_id=$1::uuid`, result.RequestID) != 2 {
		t.Fatal("request and items were not stored correctly")
	}
	execSQL(t, catalogPool, `UPDATE catalog.products SET name='Changed', unit_price_rub=9999 WHERE product_id=$1::uuid`, input.Items[0].ProductID)
	execSQL(t, catalogPool, `DELETE FROM catalog.products WHERE product_id=$1::uuid`, input.Items[1].ProductID)
	var name string
	var price int64
	if err := adminPool.QueryRow(ctx, `SELECT name, unit_price_rub FROM requests.request_items
		WHERE request_id=$1::uuid AND product_id=$2::uuid`, result.RequestID, input.Items[0].ProductID).Scan(&name, &price); err != nil {
		t.Fatal(err)
	}
	if name != "Synthetic product 0" || price != 1000 {
		t.Fatal("catalog update changed history")
	}
	// Simulate a lost response: retry after unrelated catalog changes/deletion.
	input.Items[0], input.Items[1] = input.Items[1], input.Items[0]
	replayed, err := store.Create(ctx, key, input)
	if err != nil || replayed.RequestID != result.RequestID || replayed.Code != "created" {
		t.Fatalf("replay=%+v, err=%v", replayed, err)
	}
	input.CustomerName = "Different Buyer"
	if _, err := store.Create(ctx, key, input); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("different body: %v", err)
	}
}

func TestConcurrentSameKey(t *testing.T) {
	ctx := testContext(t)
	input, key := fixture(t), newUUID()
	store := New(requestsPool)
	const clients = 16
	results := make([]Result, clients)
	errs := make([]error, clients)
	start := make(chan struct{})
	var group sync.WaitGroup
	for i := range clients {
		group.Go(func() {
			<-start
			results[i], errs[i] = store.Create(ctx, key, input)
		})
	}
	close(start)
	group.Wait()
	for i := range clients {
		if errs[i] != nil || results[i].Code != "created" || results[i].RequestID != results[0].RequestID {
			t.Fatalf("client %d: result=%+v, err=%v", i, results[i], errs[i])
		}
	}
	if count(t, `SELECT count(*) FROM requests.requests WHERE request_id=$1::uuid`, results[0].RequestID) != 1 ||
		count(t, `SELECT count(*) FROM requests.request_items WHERE request_id=$1::uuid`, results[0].RequestID) != 2 ||
		count(t, `SELECT count(*) FROM requests.request_items WHERE product_id=$1::uuid`, input.Items[0].ProductID) != 1 {
		t.Fatal("concurrent retry created duplicates")
	}
}

func TestConcurrentDifferentBodiesAndIndependentKeys(t *testing.T) {
	ctx := testContext(t)
	input, key := fixture(t), newUUID()
	other := input
	other.Comment = "Different intent"
	store := New(requestsPool)
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, body := range []Submission{input, other} {
		go func() {
			<-start
			_, err := store.Create(ctx, key, body)
			errs <- err
		}()
	}
	close(start)
	var success, conflicts int
	for range 2 {
		switch err := <-errs; {
		case err == nil:
			success++
		case errors.Is(err, ErrIdempotencyConflict):
			conflicts++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflicts=%d", success, conflicts)
	}
	first, err := store.Create(ctx, newUUID(), input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Create(ctx, newUUID(), input)
	if err != nil || first.RequestID == second.RequestID {
		t.Fatal("independent keys must create independent requests")
	}
}

func TestBusinessConflictsAreStoredWithoutRequest(t *testing.T) {
	ctx := testContext(t)
	store := New(requestsPool)
	input, key := fixture(t), newUUID()
	input.Items[0].ExpectedUnitPriceRub = 900
	result, err := store.Create(ctx, key, input)
	if err != nil || result.Code != "price_changed" || result.Quote == nil || result.Quote.TotalRub != 8000 {
		t.Fatalf("price result=%+v, err=%v", result, err)
	}
	if count(t, `SELECT count(*) FROM requests.request_submissions WHERE idempotency_key=$1::uuid AND request_id IS NULL AND response IS NOT NULL`, key) != 1 {
		t.Fatal("price conflict was not persisted without a request")
	}
	execSQL(t, catalogPool, `UPDATE catalog.products SET unit_price_rub=1100 WHERE product_id=$1::uuid`, input.Items[0].ProductID)
	replay, err := store.Create(ctx, key, input)
	if err != nil || replay.Quote == nil || replay.Quote.TotalRub != 8000 {
		t.Fatal("price conflict replay did not preserve original quote")
	}
	input.Items[0].ExpectedUnitPriceRub = 1100
	confirmed, err := store.Create(ctx, newUUID(), input)
	if err != nil || confirmed.Code != "created" {
		t.Fatalf("confirmed=%+v, err=%v", confirmed, err)
	}
	execSQL(t, catalogPool, `UPDATE catalog.products SET is_available=false WHERE product_id=$1::uuid`, input.Items[0].ProductID)
	unavailable, err := store.Create(ctx, newUUID(), input)
	if err != nil || unavailable.Code != "product_unavailable" {
		t.Fatalf("unavailable=%+v, err=%v", unavailable, err)
	}
	input.Items[0].ProductID = newUUID()
	missing, err := store.Create(ctx, newUUID(), input)
	if err != nil || missing.Code != "invalid_items" {
		t.Fatalf("missing=%+v, err=%v", missing, err)
	}
}

func TestFailureRollsBackHeaderItemsAndKey(t *testing.T) {
	ctx := testContext(t)
	input, key := fixture(t), newUUID()
	// Fail on the second (sorted) item, after the header and first item were written.
	normalized, _, err := normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	badID := normalized.Items[1].ProductID
	execSQL(t, adminPool, `CREATE FUNCTION requests.test_fail_item() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'synthetic item failure'; END $$`)
	execSQL(t, adminPool, fmt.Sprintf(`CREATE TRIGGER test_fail BEFORE INSERT ON requests.request_items
		FOR EACH ROW WHEN (NEW.product_id = '%s'::uuid) EXECUTE FUNCTION requests.test_fail_item()`, badID))
	defer execSQL(t, adminPool, `DROP TRIGGER IF EXISTS test_fail ON requests.request_items; DROP FUNCTION requests.test_fail_item()`)
	beforeHeaders := count(t, `SELECT count(*) FROM requests.requests`)
	beforeItems := count(t, `SELECT count(*) FROM requests.request_items`)
	store := New(requestsPool)
	if _, err := store.Create(ctx, key, input); err == nil {
		t.Fatal("expected injected failure")
	}
	if count(t, `SELECT count(*) FROM requests.requests`) != beforeHeaders ||
		count(t, `SELECT count(*) FROM requests.request_items`) != beforeItems ||
		count(t, `SELECT count(*) FROM requests.request_submissions WHERE idempotency_key=$1::uuid`, key) != 0 {
		t.Fatal("partial request or key survived rollback")
	}
	execSQL(t, adminPool, `DROP TRIGGER test_fail ON requests.request_items`)
	result, err := store.Create(ctx, key, input)
	if err != nil || result.Code != "created" {
		t.Fatalf("retry after rollback: result=%+v, err=%v", result, err)
	}
}

func TestRuntimePrivileges(t *testing.T) {
	ctx := testContext(t)
	input := fixture(t)
	for _, sql := range []string{
		`SELECT * FROM catalog.products`,
		`UPDATE catalog.products SET name='illegal'`,
		`UPDATE catalog.request_products_v1 SET name='illegal'`,
		`CREATE TABLE requests.illegal (id int)`,
		`CREATE TABLE catalog.illegal (id int)`,
		`CREATE TABLE public.illegal (id int)`,
		`SELECT * FROM requests.goose_db_version`,
	} {
		_, err := requestsPool.Exec(ctx, sql)
		var pgerr *pgconn.PgError
		if !errors.As(err, &pgerr) || pgerr.Code != "42501" {
			t.Fatalf("expected permission denied for %s, got %v", sql, err)
		}
	}
	if _, err := catalogPool.Exec(ctx, `SELECT * FROM requests.requests`); err == nil {
		t.Fatal("catalog can access requests")
	}
	var n int
	if err := requestsPool.QueryRow(ctx, `SELECT count(*) FROM catalog.request_products_v1 WHERE product_id=$1::uuid`, input.Items[0].ProductID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("read contract unavailable: %v", err)
	}
}

func TestCancelledContextDoesNotClaimKey(t *testing.T) {
	input, key := fixture(t), newUUID()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New(requestsPool).Create(ctx, key, input); err == nil {
		t.Fatal("cancelled context unexpectedly succeeded")
	}
	if count(t, `SELECT count(*) FROM requests.request_submissions WHERE idempotency_key=$1::uuid`, key) != 0 {
		t.Fatal("cancelled operation left a key")
	}
}

func TestInvalidInputDoesNotWriteAndConnectionFailureIsNotSuccess(t *testing.T) {
	ctx := testContext(t)
	input := fixture(t)
	before := count(t, `SELECT count(*) FROM requests.request_submissions`)
	if _, err := New(requestsPool).Create(ctx, "not-a-uuid", input); !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("invalid key: %v", err)
	}
	invalid := input
	invalid.Phone = "not a phone"
	if _, err := New(requestsPool).Create(ctx, newUUID(), invalid); !errors.Is(err, ErrInvalidSubmission) {
		t.Fatalf("invalid contacts: %v", err)
	}
	if count(t, `SELECT count(*) FROM requests.request_submissions`) != before {
		t.Fatal("validation failure wrote a submission key")
	}
	config := requestsPool.Config()
	// A separate closed pool models failure to acquire a usable DB connection
	// without stopping PostgreSQL or disturbing other local services.
	unavailable, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	unavailable.Close()
	result, err := New(unavailable).Create(ctx, newUUID(), input)
	if err == nil || result.Code != "" || result.RequestID != "" {
		t.Fatalf("connection failure looked successful: %+v, %v", result, err)
	}
}

func TestRoleAndMigrationOwnership(t *testing.T) {
	ctx := testContext(t)
	var current string
	if err := requestsPool.QueryRow(ctx, "SELECT current_user").Scan(&current); err != nil || current != os.Getenv("TIRESHOP_TEST_DB")+"_requests" {
		t.Fatalf("wrong effective role: %q, %v", current, err)
	}
	for _, table := range []string{"catalog.products", "requests.requests", "requests.request_items", "requests.request_submissions"} {
		var owner string
		if err := adminPool.QueryRow(ctx, `SELECT pg_get_userbyid(relowner) FROM pg_class WHERE oid=$1::regclass`, table).Scan(&owner); err != nil || owner != os.Getenv("TIRESHOP_TEST_DB")+"_migrator" {
			t.Fatalf("wrong migration owner for %s: %q, %v", table, owner, err)
		}
	}
	// The runtime role cannot assume the migrator through inheritance. Tests use
	// SET ROLE from the local admin login only to exercise its effective SQL grants.
	var super, createDB, createRole bool
	if err := requestsPool.QueryRow(ctx, `SELECT rolsuper, rolcreatedb, rolcreaterole FROM pg_roles WHERE rolname=current_user`).Scan(&super, &createDB, &createRole); err != nil || super || createDB || createRole {
		t.Fatalf("runtime role is privileged: %v", err)
	}
}
