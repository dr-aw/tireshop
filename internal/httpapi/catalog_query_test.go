package httpapi

import (
	"net/url"
	"strings"
	"testing"
)

func TestCatalogQueryRejectsInvalidFilters(t *testing.T) {
	for _, query := range []string{"kind=truck", "limit=101", "offset=-1", "brand=a&brand=b", "unknown=x", "sort=review_notes", "direction=sideways", "price_min=10&price_max=9", "tread_min=101", "diameter=NaN", "diameter=Inf", "diameter=0", "production_year=2020.5", "kind=tyre&pcd=6x222", "kind=wheel&tread_min=10", "visibility=hidden", "price_min=1.5"} {
		t.Run(query, func(t *testing.T) {
			values, _ := url.ParseQuery(query)
			if _, err := parseCatalogQuery(values, false); err == nil {
				t.Fatal("accepted invalid filters")
			}
		})
	}
}
func TestCatalogQueryBindsInput(t *testing.T) {
	values := url.Values{"q": {"50%_'; DROP TABLE catalog.products; --"}, "brand": {"'; SELECT 1; --"}, "sort": {"unit_price_rub"}, "direction": {"desc"}}
	q, err := parseCatalogQuery(values, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(q.Where, "DROP") || strings.Contains(q.Where, "SELECT") || !strings.Contains(q.Where, "p.is_available") {
		t.Fatal(q.Where)
	}
	if q.Order != "p.unit_price_rub desc NULLS LAST, p.product_id ASC" {
		t.Fatal(q.Order)
	}
	if q.Args[0] != `%50\%\_'; DROP TABLE catalog.products; --%` {
		t.Fatal(q.Args[0])
	}
}
