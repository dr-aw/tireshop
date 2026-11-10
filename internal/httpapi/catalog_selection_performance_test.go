//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

// Opt-in measurements on an isolated synthetic DB, never on development data.
func TestSelectionPerformance(t *testing.T) {
	if os.Getenv("TIRESHOP_SELECTION_BENCH") != "1" {
		t.Skip("Run mise run test:selection-performance")
	}
	pool, _ := apiPools(t)
	db := adminDB(t)
	t.Logf("CPU=%d GOMAXPROCS=%d pool=%d", runtime.NumCPU(), runtime.GOMAXPROCS(0), pool.Config().MaxConns)
	for _, file := range []string{"/sys/fs/cgroup/cpu.max", "/sys/fs/cgroup/memory.max"} {
		if b, e := os.ReadFile(file); e == nil {
			t.Logf("%s: %s", file, b)
		}
	}
	t.Cleanup(func() {
		_, _ = db.Exec(context.Background(), `DELETE FROM catalog.product_images WHERE product_id IN (SELECT product_id FROM catalog.products WHERE brand='Selection benchmark'); DELETE FROM catalog.products WHERE brand='Selection benchmark'`)
	})
	server := httptest.NewServer(Catalog(pool))
	defer server.Close()
	for _, count := range []int{2000, 20000} {
		_, err := db.Exec(t.Context(), `DELETE FROM catalog.product_images WHERE product_id IN (SELECT product_id FROM catalog.products WHERE brand='Selection benchmark'); DELETE FROM catalog.products WHERE brand='Selection benchmark';`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(t.Context(), `INSERT INTO catalog.products(product_id,name,kind,brand,stock,units_per_sale,unit_price_rub,is_available)
 SELECT md5('selection-benchmark-'||n)::uuid,'Selection benchmark '||n,CASE WHEN n%4=0 THEN 'wheel' ELSE 'tyre' END,'Selection benchmark',n%17,CASE WHEN n%4=0 THEN 4 ELSE 1 END,1000+n,n%13<>0 FROM generate_series(1,$1::int) n`, count)
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(t.Context(), `INSERT INTO catalog.tyre_specs(product_id,size,season,tread_percent)
 SELECT product_id,CASE WHEN stock%7=0 THEN '205/60R19.5' WHEN stock%3=0 THEN '205/60R17.5' ELSE '195/70R17.5' END,'Лето',80 FROM catalog.products WHERE brand='Selection benchmark' AND kind='tyre';
 INSERT INTO catalog.wheel_specs(product_id,diameter_inches,width_inches,pcd,offset_mm,center_bore_mm)
 SELECT product_id,CASE WHEN stock%7=0 THEN 19.5 ELSE 17.5 END,CASE WHEN stock%3=0 THEN 6 ELSE 6.75 END,'6x222.25',-10,164 FROM catalog.products WHERE brand='Selection benchmark' AND kind='wheel';
 INSERT INTO catalog.product_images(product_id,position,source_path,source_sha256,webp_path,width,height)
 SELECT product_id,0,'synthetic',product_id::text,'/media/synthetic.webp',1280,1280 FROM catalog.products WHERE brand='Selection benchmark' AND stock%2=0;
 ANALYZE catalog.products; ANALYZE catalog.tyre_specs; ANALYZE catalog.wheel_specs; ANALYZE catalog.product_images;`)
		if err != nil {
			t.Fatal(err)
		}
		for _, indexed := range []bool{false, true} {
			ddl := `DROP INDEX catalog.tyre_specs_selection_idx; DROP INDEX catalog.wheel_specs_selection_idx;`
			if indexed {
				ddl = `CREATE INDEX tyre_specs_selection_idx ON catalog.tyre_specs(radius,size_key,product_id); CREATE INDEX wheel_specs_selection_idx ON catalog.wheel_specs(radius,width_inches,product_id);`
			}
			if _, err := db.Exec(t.Context(), ddl); err != nil {
				t.Fatal(err)
			}
			cases := []string{"kind=tyre&radius=17.5", "kind=tyre&radius=19.5&tyre_size=205%2F60R19.5", "kind=tyre&radius=17.5&offset=960", "kind=tyre&radius=17.5&sort=unit_price_rub", "kind=wheel&radius=17.5&width=6.75&pcd=6x222.25&in_stock=true"}
			for _, query := range cases {
				values, _ := url.ParseQuery(query + "&brand=Selection+benchmark")
				q, err := parseCatalogQuery(values, false)
				if err != nil {
					t.Fatal(err)
				}
				type statement struct {
					name, sql string
					args      []any
				}
				pageSQL, pageArgs := publicCatalogPageQuery(q)
				statements := []statement{{"count", "SELECT count(*) FROM catalog.product_details p WHERE " + q.Where, q.Args}, {"list", pageSQL, pageArgs}}
				for _, key := range []string{"offset", "sort"} {
					values.Del(key)
				}
				sql, args, err := selectionQuery(values)
				if err != nil {
					t.Fatal(err)
				}
				statements = append(statements, statement{"selection", sql, args})
				for _, st := range statements {
					var plan []byte
					if err := pool.QueryRow(t.Context(), "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+st.sql, st.args...).Scan(&plan); err != nil {
						t.Fatal(err)
					}
					var parsed []map[string]any
					if err := json.Unmarshal(plan, &parsed); err != nil {
						t.Fatal(err)
					}
					t.Logf("n=%d indexed=%v %s %s cold/first=%.3fms plan=%s", count, indexed, st.name, query, parsed[0]["Execution Time"], plan)
					times := make([]float64, 0, 20)
					for i := 0; i < 20; i++ {
						start := time.Now()
						rows, err := pool.Query(t.Context(), st.sql, st.args...)
						if err != nil {
							t.Fatal(err)
						}
						for rows.Next() {
						}
						err = rows.Err()
						rows.Close()
						if err != nil {
							t.Fatal(err)
						}
						times = append(times, float64(time.Since(start).Microseconds())/1000)
					}
					sort.Float64s(times)
					t.Logf("SQL n=%d indexed=%v %s %s p50=%.2f p95=%.2f p99=%.2f ms", count, indexed, st.name, query, times[9], times[18], times[19])
					if indexed && times[18] > 50 {
						t.Errorf("SQL p95 exceeds 50ms: %s %s %.2f", st.name, query, times[18])
					}
				}
			}
		}
		for _, path := range []string{"/api/products?kind=tyre&radius=17.5&brand=Selection+benchmark", "/api/products/selection?kind=tyre&radius=17.5&brand=Selection+benchmark", "/api/products/selection?kind=wheel&radius=17.5&pcd=6x222.25&brand=Selection+benchmark"} {
			var wg sync.WaitGroup
			var mu sync.Mutex
			times := []float64{}
			errors := 0
			for worker := 0; worker < 10; worker++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < 30; i++ {
						start := time.Now()
						res, err := http.Get(server.URL + path)
						ok := err == nil
						if ok {
							_, err = io.Copy(io.Discard, res.Body)
							res.Body.Close()
							ok = err == nil && res.StatusCode == 200
						}
						ms := float64(time.Since(start).Microseconds()) / 1000
						mu.Lock()
						times = append(times, ms)
						if !ok {
							errors++
						}
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			sort.Float64s(times)
			t.Logf("HTTP n=%d concurrent=10 requests=%d errors=%d %s p50=%.2f p95=%.2f p99=%.2f ms", count, len(times), errors, path, times[149], times[284], times[296])
			if errors > 0 || times[284] > 150 {
				t.Errorf("HTTP budget exceeded: errors=%d p95=%.2f", errors, times[284])
			}
		}
		rows, err := db.Query(t.Context(), `SELECT indexrelname,pg_relation_size(indexrelid) FROM pg_stat_user_indexes WHERE indexrelname IN ('tyre_specs_selection_idx','wheel_specs_selection_idx') ORDER BY indexrelname`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var name string
			var size int64
			rows.Scan(&name, &size)
			t.Logf("n=%d index=%s bytes=%d", count, name, size)
		}
		rows.Close()
		start := time.Now()
		_, err = db.Exec(t.Context(), `UPDATE catalog.tyre_specs SET size=CASE WHEN size='195/70R17.5' THEN '205/60R17.5' ELSE '195/70R17.5' END WHERE product_id IN (SELECT product_id FROM catalog.products WHERE brand='Selection benchmark' AND kind='tyre' LIMIT 1000)`)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("n=%d update 1000 sizes with generated fields/index: %s", count, time.Since(start))
	}
}
