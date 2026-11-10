package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type selectionOption struct {
	Value        string `json:"value"`
	Label        string `json:"label"`
	ProductCount int64  `json:"product_count"`
	// Decimal string: even a single permitted stock * sale unit can exceed JS precision.
	UnitCount string `json:"unit_count"`
}
type selectionResponse struct {
	Diameters []selectionOption `json:"diameters"`
	Radii     []selectionOption `json:"radii"`
	Sizes     []selectionOption `json:"sizes"`
}

// One statement provides both aggregates from the same database snapshot.
func selectionQuery(values url.Values) (string, []any, error) {
	if values.Get("kind") == "" {
		return "", nil, fmt.Errorf("Выберите категорию")
	}
	for _, key := range []string{"sort", "direction", "limit", "offset"} {
		if _, ok := values[key]; ok {
			return "", nil, fmt.Errorf("Некорректные параметры подбора")
		}
	}
	if _, err := parseCatalogQuery(values, false); err != nil {
		return "", nil, err
	}
	base := url.Values{}
	for key, v := range values {
		base[key] = append([]string(nil), v...)
	}
	for _, key := range []string{"radius", "rim_diameter", "diameter", "tyre_size", "size"} {
		base.Del(key)
	}
	wheel := values.Get("kind") == "wheel"
	if wheel {
		base.Del("width")
	}
	q, err := parseCatalogQuery(base, false)
	if err != nil {
		return "", nil, err
	}
	table := "catalog.tyre_specs"
	sizeValue, sizeLabel := "s.size_key", "s.size_key"
	unit, width, profile := "s.width_unit", "s.width_value", "s.aspect_ratio"
	sizePresent := "s.size_key IS NOT NULL"
	diameterLabel := "'R' || trim_scale(s.diameter)::text"
	if wheel {
		table = "catalog.wheel_specs"
		diameterLabel = "trim_scale(s.diameter)::text || '″'"
		sizeValue = "trim_scale(s.width_inches)::text"
		sizeLabel = sizeValue + " || '×' || trim_scale(s.diameter)::text"
		unit, width, profile = "'in'::text", "s.width_inches", "NULL::numeric"
		sizePresent = "s.width_inches IS NOT NULL"
	}
	diameterConditions := []string{}
	for _, key := range []string{"radius", "rim_diameter", "diameter"} {
		if value := values.Get(key); value != "" {
			q.Args = append(q.Args, value) // validated above; retain exact decimal precision
			diameterConditions = append(diameterConditions, fmt.Sprintf("s.diameter=$%d", len(q.Args)))
		}
	}
	diameterWhere := "FALSE"
	if len(diameterConditions) > 0 {
		diameterWhere = strings.Join(diameterConditions, " AND ")
	}
	from := " FROM catalog.product_details p JOIN " + table + " s USING(product_id) WHERE " + q.Where
	count := "count(*) AS product_count, sum(p.stock::bigint * p.units_per_sale::bigint)::text AS unit_count"
	sql := `SELECT section,value,label,product_count,unit_count FROM (
 SELECT 0 AS section,trim_scale(s.diameter)::text AS value,` + diameterLabel + ` AS label,` + count + `,
 ''::text AS sort_unit,s.diameter AS sort_width,NULL::numeric AS sort_profile` + from + ` AND s.diameter IS NOT NULL GROUP BY s.diameter
 UNION ALL
 SELECT 1,` + sizeValue + `,` + sizeLabel + `,` + count + `,` + unit + `,` + width + `,` + profile + from + ` AND ` + diameterWhere + ` AND ` + sizePresent + `
 GROUP BY s.diameter,` + sizeValue + `,` + unit + `,` + width + `,` + profile + `
 ) choices ORDER BY section,sort_unit,sort_width,sort_profile NULLS FIRST,value`
	return sql, q.Args, nil
}

func catalogSelection(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sql, args, err := selectionQuery(r.URL.Query())
		if err != nil {
			fail(w, 400, "validation_error", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		rows, err := pool.Query(ctx, sql, args...)
		if err != nil {
			fail(w, 503, "unavailable", "Подбор временно недоступен")
			return
		}
		defer rows.Close()
		response := selectionResponse{Radii: []selectionOption{}, Sizes: []selectionOption{}}
		for rows.Next() {
			var section int
			var option selectionOption
			if err := rows.Scan(&section, &option.Value, &option.Label, &option.ProductCount, &option.UnitCount); err != nil {
				fail(w, 503, "unavailable", "Подбор временно недоступен")
				return
			}
			if section == 0 {
				response.Radii = append(response.Radii, option)
			} else {
				response.Sizes = append(response.Sizes, option)
			}
		}
		if rows.Err() != nil {
			fail(w, 503, "unavailable", "Подбор временно недоступен")
			return
		}
		response.Diameters = response.Radii
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, 200, response)
	}
}
