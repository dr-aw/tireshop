package requests

import (
	"errors"
	"math"
	"testing"
)

func TestQuoteSnapshotsCatalogAndCalculatesRubles(t *testing.T) {
	products := map[string]Product{
		"tire": {Name: "Test tire", UnitPriceRub: 12000, Available: true},
		"rim":  {Name: "Test rim", UnitPriceRub: 5000, Available: true},
	}
	inputs := []RequestedItem{{"tire", 4, 12000}, {"rim", 2, 5000}}
	quote, err := PrepareQuote(inputs, products)
	if err != nil {
		t.Fatal(err)
	}
	if quote.TotalRub != 58000 || len(quote.Items) != 2 || quote.Items[0].TotalRub != 48000 {
		t.Fatalf("unexpected quote: %+v", quote)
	}
	products["tire"] = Product{Name: "Changed", UnitPriceRub: 1}
	inputs[0].Quantity = 100
	if quote.Items[0].Name != "Test tire" || quote.Items[0].UnitPriceRub != 12000 || quote.Items[0].Quantity != 4 {
		t.Fatalf("snapshot changed with source: %+v", quote)
	}
}

func TestPriceChangeReturnsWholeCurrentQuote(t *testing.T) {
	products := map[string]Product{
		"tire": {Name: "Test tire", UnitPriceRub: 12000, Available: true},
		"rim":  {Name: "Test rim", UnitPriceRub: 5000, Available: true},
	}
	quote, err := PrepareQuote([]RequestedItem{{"tire", 2, 10000}, {"rim", 1, 6000}}, products)
	if !errors.Is(err, ErrPriceChanged) || quote.TotalRub != 29000 || len(quote.Items) != 2 {
		t.Fatalf("quote=%+v, err=%v", quote, err)
	}
	// Confirmation must be checked again; it is not a permanent price guarantee.
	confirmed := []RequestedItem{{"tire", 2, 12000}, {"rim", 1, 5000}}
	if _, err := PrepareQuote(confirmed, products); err != nil {
		t.Fatal(err)
	}
	products["tire"] = Product{Name: "Test tire", UnitPriceRub: 13000, Available: true}
	if _, err := PrepareQuote(confirmed, products); !errors.Is(err, ErrPriceChanged) {
		t.Fatalf("second price change was not detected: %v", err)
	}
}

func TestInvalidQuoteNeverReturnsPartialBasket(t *testing.T) {
	tests := []struct {
		name    string
		items   []RequestedItem
		product Product
		wantErr error
	}{
		{"empty", nil, Product{}, ErrInvalidItems},
		{"blank ID", []RequestedItem{{" ", 1, 10}}, Product{}, ErrInvalidItems},
		{"unknown", []RequestedItem{{"unknown", 1, 10}}, Product{}, ErrInvalidItems},
		{"duplicate", []RequestedItem{{"p", 1, 10}, {"p", 1, 10}}, Product{"P", 10, true}, ErrInvalidItems},
		{"zero quantity", []RequestedItem{{"p", 0, 10}}, Product{"P", 10, true}, ErrInvalidItems},
		{"negative quantity", []RequestedItem{{"p", -1, 10}}, Product{"P", 10, true}, ErrInvalidItems},
		{"unsafe quantity", []RequestedItem{{"p", math.MaxInt64, 10}}, Product{"P", 10, true}, ErrInvalidItems},
		{"negative expected price", []RequestedItem{{"p", 1, -1}}, Product{"P", 10, true}, ErrInvalidItems},
		{"unsafe expected price", []RequestedItem{{"p", 1, MaxAmount + 1}}, Product{"P", 10, true}, ErrInvalidItems},
		{"unavailable", []RequestedItem{{"p", 1, 10}}, Product{"P", 10, false}, ErrProductUnavailable},
		{"negative price", []RequestedItem{{"p", 1, 10}}, Product{"P", -1, true}, ErrInvalidProduct},
		{"unsafe price", []RequestedItem{{"p", 1, 10}}, Product{"P", math.MaxInt64, true}, ErrInvalidProduct},
		{"blank name", []RequestedItem{{"p", 1, 10}}, Product{" ", 10, true}, ErrInvalidProduct},
		{"multiply overflow", []RequestedItem{{"p", MaxAmount, MaxAmount}}, Product{"P", MaxAmount, true}, ErrAmountOverflow},
		{"sum overflow", []RequestedItem{{"p", 1, MaxAmount}, {"q", 1, 1}}, Product{"P", MaxAmount, true}, ErrAmountOverflow},
		{"late unavailable", []RequestedItem{{"q", 1, 1}, {"p", 1, 10}}, Product{"P", 10, false}, ErrProductUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote, err := PrepareQuote(tt.items, map[string]Product{"p": tt.product, "q": {"Q", 1, true}})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
			if len(quote.Items) != 0 || quote.TotalRub != 0 {
				t.Fatalf("partial quote returned: %+v", quote)
			}
		})
	}
}

func TestQuoteExactAmountBoundary(t *testing.T) {
	quote, err := PrepareQuote([]RequestedItem{{"p", 1, MaxAmount}}, map[string]Product{"p": {"P", MaxAmount, true}})
	if err != nil || quote.TotalRub != MaxAmount {
		t.Fatalf("quote=%+v, err=%v", quote, err)
	}
}
