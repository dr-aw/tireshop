// Package requests implements the business rules for customer requests.
// Persistence, authorization and HTTP validation belong to the service using it.
package requests

import (
	"errors"
	"fmt"
	"strings"
)

// MaxAmount is the largest integer exactly representable by JSON clients using
// JavaScript numbers. Prices, quantities and totals must stay within this bound.
const MaxAmount int64 = 1<<53 - 1

var (
	ErrInvalidItems       = errors.New("invalid request items")
	ErrProductUnavailable = errors.New("product unavailable")
	ErrInvalidProduct     = errors.New("invalid catalog product")
	ErrAmountOverflow     = errors.New("amount exceeds supported range")
	ErrPriceChanged       = errors.New("price changed")
)

// RequestedItem contains the quantity and price the customer confirmed.
// ExpectedUnitPriceRub is not trusted as the actual catalog price.
type RequestedItem struct {
	ProductID            string
	Quantity             int64
	ExpectedUnitPriceRub int64
}

// Product is the minimal catalog data needed to prepare a request.
// Products are supplied by ID in PrepareQuote's map argument.
type Product struct {
	Name         string
	UnitPriceRub int64
	Available    bool
}

// Item is a value snapshot, independent of subsequent changes to the catalog.
type Item struct {
	ProductID    string
	Name         string
	Quantity     int64
	UnitPriceRub int64
	TotalRub     int64
}

type Quote struct {
	Items    []Item
	TotalRub int64
}

// PrepareQuote validates a basket and calculates its current catalog prices.
// On ErrPriceChanged it returns the complete updated quote for confirmation.
// Every other error returns an empty quote, never a partial basket.
// It does not reserve stock or persist a request.
func PrepareQuote(requested []RequestedItem, products map[string]Product) (Quote, error) {
	if len(requested) == 0 {
		return Quote{}, ErrInvalidItems
	}

	quote := Quote{Items: make([]Item, 0, len(requested))}
	seen := make(map[string]bool, len(requested))
	priceChanged := false
	for _, input := range requested {
		if strings.TrimSpace(input.ProductID) == "" || seen[input.ProductID] ||
			input.Quantity <= 0 || input.Quantity > MaxAmount ||
			input.ExpectedUnitPriceRub < 0 || input.ExpectedUnitPriceRub > MaxAmount {
			return Quote{}, ErrInvalidItems
		}
		seen[input.ProductID] = true

		product, exists := products[input.ProductID]
		if !exists {
			return Quote{}, fmt.Errorf("%w: unknown product", ErrInvalidItems)
		}
		if !product.Available {
			return Quote{}, ErrProductUnavailable
		}
		if strings.TrimSpace(product.Name) == "" || product.UnitPriceRub < 0 || product.UnitPriceRub > MaxAmount {
			return Quote{}, ErrInvalidProduct
		}
		// Check before multiplying, so even int64 cannot overflow.
		if product.UnitPriceRub > MaxAmount/input.Quantity {
			return Quote{}, ErrAmountOverflow
		}
		lineTotal := product.UnitPriceRub * input.Quantity
		if quote.TotalRub > MaxAmount-lineTotal {
			return Quote{}, ErrAmountOverflow
		}
		quote.Items = append(quote.Items, Item{
			ProductID: input.ProductID, Name: product.Name,
			Quantity: input.Quantity, UnitPriceRub: product.UnitPriceRub,
			TotalRub: lineTotal,
		})
		quote.TotalRub += lineTotal
		priceChanged = priceChanged || input.ExpectedUnitPriceRub != product.UnitPriceRub
	}
	if priceChanged {
		return quote, ErrPriceChanged
	}
	return quote, nil
}
