package postgres

import (
	"crypto/sha256"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"tireshop/internal/requests"
)

var (
	ErrInvalidSubmission   = errors.New("invalid submission")
	ErrIdempotencyConflict = errors.New("idempotency key already used with different data")
	ErrIncompleteResult    = errors.New("stored submission has no completed result")
)

const MaxItems = 100
const MaxQuantity int64 = 10000

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

// Submission is input to the storage operation, not the HTTP wire format.
// HTTP handlers will enforce JSON/body limits before calling the store.
type Submission struct {
	CustomerName string
	Phone        string
	Comment      string
	Items        []requests.RequestedItem
}

// Result is stored with the idempotency key so retries reproduce the original
// business result. Code is created, price_changed, product_unavailable or invalid_items.
// Infrastructure failures are returned as Go errors and are not persisted.
type Result struct {
	Code      string          `json:"code"`
	RequestID string          `json:"request_id,omitempty"`
	Quote     *requests.Quote `json:"quote,omitempty"`
}

func normalize(input Submission) (Submission, [32]byte, error) {
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.Comment = strings.TrimSpace(input.Comment)
	input.Phone = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(input.Phone))
	if !validText(input.CustomerName, 1, 100) || !validText(input.Comment, 0, 2000) ||
		!phonePattern.MatchString(input.Phone) || len(input.Items) == 0 || len(input.Items) > MaxItems {
		return Submission{}, [32]byte{}, ErrInvalidSubmission
	}
	// Clone before sorting/canonicalizing, so callers retain ownership of their input.
	input.Items = slices.Clone(input.Items)
	for i := range input.Items {
		item := &input.Items[i]
		if !uuidPattern.MatchString(item.ProductID) || item.Quantity <= 0 || item.Quantity > MaxQuantity ||
			item.ExpectedUnitPriceRub < 0 || item.ExpectedUnitPriceRub > requests.MaxAmount {
			return Submission{}, [32]byte{}, ErrInvalidSubmission
		}
		item.ProductID = strings.ToLower(item.ProductID)
	}
	slices.SortFunc(input.Items, func(a, b requests.RequestedItem) int {
		return strings.Compare(a.ProductID, b.ProductID)
	})
	for i := 1; i < len(input.Items); i++ {
		if input.Items[i-1].ProductID == input.Items[i].ProductID {
			return Submission{}, [32]byte{}, ErrInvalidSubmission
		}
	}
	// A fixed struct (no map) and sorted items give a deterministic representation.
	// Keep this representation stable when evolving the submission contract.
	// Fingerprints are persisted contracts. Keep v1's escaping and formatting
	// even though the implementation now uses json/v2; otherwise old keys with
	// characters such as '<' or '&' would incorrectly conflict after an upgrade.
	payload, err := json.Marshal(input, jsonv1.DefaultOptionsV1())
	if err != nil {
		return Submission{}, [32]byte{}, err
	}
	return input, sha256.Sum256(payload), nil
}

func validText(value string, min, max int) bool {
	count := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && !strings.ContainsRune(value, 0) && count >= min && count <= max
}
