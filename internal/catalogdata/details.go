// Package catalogdata defines the catalog-owned product attributes.
package catalogdata

import (
	"encoding/json/v2"
	"fmt"
	"strings"
)

type Details struct {
	SKU            string   `json:"sku"`
	Kind           string   `json:"kind"`
	Description    string   `json:"description"`
	Brand          string   `json:"brand"`
	Model          string   `json:"model"`
	ProductionYear *int     `json:"production_year"`
	Size           string   `json:"size"`
	Season         string   `json:"season"`
	TreadPercent   *float64 `json:"tread_percent"`
	LoadIndex      string   `json:"load_index"`
	SpeedIndex     string   `json:"speed_index"`
	Location       string   `json:"location"`
	Condition      string   `json:"condition"`
	Diameter       *float64 `json:"diameter"`
	Width          *float64 `json:"width"`
	WheelOffset    *float64 `json:"wheel_offset"`
	PCD            string   `json:"pcd"`
	WheelType      string   `json:"wheel_type"`
	CenterBore     *float64 `json:"center_bore"`
	UnitsPerSale   int      `json:"units_per_sale"`
	ReviewNotes    string   `json:"review_notes"`
}

var Fields = []string{"sku", "kind", "description", "brand", "model", "production_year", "size", "season", "tread_percent", "load_index", "speed_index", "location", "condition", "diameter", "width", "wheel_offset", "pcd", "wheel_type", "center_bore", "units_per_sale", "review_notes"}

// SQL contains only hardcoded field names, never user input.
func Select(alias string) string {
	parts := []string{}
	for _, field := range Fields {
		parts = append(parts, "'"+field+"',"+alias+"."+field)
	}
	return "jsonb_build_object(" + strings.Join(parts, ",") + ")"
}
func UpdateSQL() string {
	parts := []string{}
	for _, field := range Fields {
		parts = append(parts, field+"=d."+field)
	}
	return `UPDATE catalog.products p SET ` + strings.Join(parts, ",") + ` FROM jsonb_populate_record(NULL::catalog.products,$2::jsonb) d WHERE p.product_id=$1::uuid`
}
func (d Details) Validate() error {
	if d.Kind != "tyre" && d.Kind != "wheel" {
		return fmt.Errorf("invalid kind")
	}
	if d.UnitsPerSale < 1 || d.UnitsPerSale > 100 {
		return fmt.Errorf("invalid sale unit")
	}
	if d.ProductionYear != nil && (*d.ProductionYear < 1900 || *d.ProductionYear > 2200) {
		return fmt.Errorf("invalid year")
	}
	if d.TreadPercent != nil && (*d.TreadPercent < 0 || *d.TreadPercent > 100) {
		return fmt.Errorf("invalid tread")
	}
	for _, v := range []*float64{d.Diameter, d.Width, d.CenterBore} {
		if v != nil && (*v <= 0 || *v > 10000) {
			return fmt.Errorf("invalid dimension")
		}
	}
	b, err := json.Marshal(d)
	if err != nil || len(b) > 24000 {
		return fmt.Errorf("attributes too large")
	}
	if len(d.SKU) > 80 {
		return fmt.Errorf("sku too long")
	}
	return nil
}
func (d Details) UnitLabel() string {
	if d.UnitsPerSale > 1 {
		return fmt.Sprintf("комплект (%d шт.)", d.UnitsPerSale)
	}
	return "шт."
}

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
