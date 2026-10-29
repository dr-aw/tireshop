package httpapi

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"tireshop/internal/catalogdata"
)

// Only these catalog-owned columns may be used in public filters or facets.
var catalogFilterFields = []string{"brand", "model", "size", "condition", "production_year", "diameter", "width", "season", "load_index", "speed_index", "wheel_offset", "pcd", "wheel_type", "center_bore"}
var numericCatalogFilters = map[string]bool{"production_year": true, "diameter": true, "width": true, "wheel_offset": true, "center_bore": true}
var tyreFilters = map[string]bool{"season": true, "load_index": true, "speed_index": true, "tread_min": true, "tread_max": true}
var wheelFilters = map[string]bool{"wheel_offset": true, "pcd": true, "wheel_type": true, "center_bore": true}

type catalogQuery struct {
	Where, Order  string
	Args          []any
	Limit, Offset int
}

func parseCatalogQuery(values url.Values, admin bool) (catalogQuery, error) {
	q := catalogQuery{Where: "TRUE", Order: "p.product_id ASC", Limit: 24}
	if admin {
		q.Limit = 50
	} else {
		q.Where = "p.is_available"
	}
	invalid := func() (catalogQuery, error) {
		return q, fmt.Errorf("Проверьте параметры фильтров и страницы")
	}
	allowed := map[string]bool{"q": true, "kind": true, "sort": true, "direction": true, "limit": true, "offset": true, "price_min": true, "price_max": true, "tread_min": true, "tread_max": true, "in_stock": true}
	for _, f := range catalogFilterFields {
		allowed[f] = true
	}
	if admin {
		allowed["visibility"] = true
		allowed["stock_min"] = true
		allowed["stock_max"] = true
	}
	for key, vals := range values {
		if !allowed[key] || len(vals) != 1 {
			return invalid()
		}
	}
	for key, target := range map[string]*int{"limit": &q.Limit, "offset": &q.Offset} {
		if s := values.Get(key); s != "" {
			n, e := strconv.Atoi(s)
			if e != nil {
				return invalid()
			}
			*target = n
		}
	}
	if q.Limit < 1 || q.Limit > 100 || q.Offset < 0 || q.Offset > 100000 {
		return invalid()
	}
	add := func(sql string, value any) {
		q.Args = append(q.Args, value)
		q.Where += " AND " + fmt.Sprintf(sql, len(q.Args))
	}
	kind := values.Get("kind")
	if kind != "" && kind != "tyre" && kind != "wheel" {
		return invalid()
	}
	if kind != "" {
		add("p.kind=$%d", kind)
	}
	for key := range values {
		if values.Get(key) != "" && ((kind == "tyre" && wheelFilters[key]) || (kind == "wheel" && tyreFilters[key])) {
			return invalid()
		}
	}
	search := strings.TrimSpace(values.Get("q"))
	if utf8.RuneCountInString(search) > 200 {
		return invalid()
	}
	if search != "" {
		// Search is a literal substring, not a user-supplied LIKE pattern.
		search = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(search)
		add("(p.name || ' ' || coalesce(p.sku,'') || ' ' || coalesce(p.size,'')) ILIKE $%d", "%"+search+"%")
	}
	for _, field := range catalogFilterFields {
		value := values.Get(field)
		if value == "" {
			continue
		}
		if utf8.RuneCountInString(value) > 200 {
			return invalid()
		}
		if numericCatalogFilters[field] {
			n, e := strconv.ParseFloat(value, 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > 10000 {
				return invalid()
			}
			if field == "production_year" && (n < 1900 || n > 2200 || n != math.Trunc(n)) {
				return invalid()
			}
			if (field == "width" || field == "diameter" || field == "center_bore") && n <= 0 {
				return invalid()
			}
			add("p."+field+"=$%d", n)
		} else {
			add("p."+field+"=$%d", value)
		}
	}
	for _, rangeField := range []struct {
		Key, Column string
		Max         float64
		Integer     bool
	}{{"price", "unit_price_rub", 1000000000, true}, {"tread", "tread_percent", 100, false}, {"stock", "stock", 1000000000, true}} {
		if rangeField.Key == "stock" && !admin {
			continue
		}
		bounds := map[string]float64{}
		for _, suffix := range []string{"min", "max"} {
			value := values.Get(rangeField.Key + "_" + suffix)
			if value == "" {
				continue
			}
			n, e := strconv.ParseFloat(value, 64)
			if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > rangeField.Max || (rangeField.Integer && n != math.Trunc(n)) {
				return invalid()
			}
			bounds[suffix] = n
			op := ">="
			if suffix == "max" {
				op = "<="
			}
			if rangeField.Integer {
				add("p."+rangeField.Column+op+"$%d", int64(n))
			} else {
				add("p."+rangeField.Column+op+"$%d", n)
			}
		}
		lo, lok := bounds["min"]
		hi, hok := bounds["max"]
		if lok && hok && lo > hi {
			return invalid()
		}
	}
	if stock := values.Get("in_stock"); stock != "" {
		if stock != "true" {
			return invalid()
		}
		q.Where += " AND p.stock>0"
	}
	if admin {
		switch values.Get("visibility") {
		case "":
		case "visible":
			q.Where += " AND p.is_available"
		case "hidden":
			q.Where += " AND NOT p.is_available"
		default:
			return invalid()
		}
	}
	sort := values.Get("sort")
	direction := values.Get("direction")
	if direction != "" && direction != "asc" && direction != "desc" {
		return invalid()
	}
	sorts := map[string]bool{"name": true, "unit_price_rub": true, "brand": true}
	if admin {
		sorts["stock"] = true
		sorts["is_available"] = true
		for _, f := range catalogdata.Fields {
			sorts[f] = true
		}
	}
	if sort != "" {
		if !sorts[sort] {
			return invalid()
		}
		if direction == "" {
			direction = "asc"
		}
		q.Order = "p." + sort + " " + direction + " NULLS LAST, p.product_id ASC"
	}
	return q, nil
}

// Facets share the same visibility scope as their corresponding list endpoint.
// They deliberately depend only on kind, so zero matches never trap a filter.
func catalogFacets(pool *pgxpool.Pool, admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for key := range r.URL.Query() {
			if key != "kind" {
				fail(w, 400, "validation_error", "Некорректные параметры справочника")
				return
			}
		}
		q, err := parseCatalogQuery(r.URL.Query(), admin)
		if err != nil {
			fail(w, 400, "validation_error", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		columns := make([]string, 0, len(catalogFilterFields))
		for _, field := range catalogFilterFields {
			columns = append(columns, "'"+field+"',p."+field)
		}
		rows, err := pool.Query(ctx, "SELECT jsonb_build_object("+strings.Join(columns, ",")+") FROM catalog.products p WHERE "+q.Where, q.Args...)
		if err != nil {
			adminError(w, err)
			return
		}
		defer rows.Close()
		sets := map[string]map[string]bool{}
		for _, f := range catalogFilterFields {
			sets[f] = map[string]bool{}
		}
		for rows.Next() {
			var raw []byte
			if err = rows.Scan(&raw); err != nil {
				adminError(w, err)
				return
			}
			var details map[string]any
			if err = json.Unmarshal(raw, &details); err != nil {
				adminError(w, err)
				return
			}
			for _, f := range catalogFilterFields {
				v := details[f]
				if v != nil {
					s := fmt.Sprint(v)
					if strings.TrimSpace(s) != "" {
						sets[f][s] = true
					}
				}
			}
		}
		if rows.Err() != nil {
			adminError(w, rows.Err())
			return
		}
		writeJSON(w, 200, map[string]any{"options": sortedFacets(sets)})
	}
}

func sortedFacets(sets map[string]map[string]bool) map[string][]string {
	result := map[string][]string{}
	for field, values := range sets {
		options := []string{}
		for v := range values {
			options = append(options, v)
		}
		sort.Slice(options, func(i, j int) bool {
			if numericCatalogFilters[field] {
				a, _ := strconv.ParseFloat(options[i], 64)
				b, _ := strconv.ParseFloat(options[j], 64)
				return a < b
			}
			return options[i] < options[j]
		})
		result[field] = options
	}
	return result
}
