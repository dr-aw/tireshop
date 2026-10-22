package postgres

import (
	"errors"
	"strings"
	"testing"

	"tireshop/internal/requests"
)

func TestNormalizeCanonicalFingerprint(t *testing.T) {
	first := Submission{
		CustomerName: "  Test Buyer  ", Phone: "+7 (900) 000-00-00", Comment: " comment ",
		Items: []requests.RequestedItem{
			{ProductID: "AAAAAAAA-0000-0000-0000-000000000002", Quantity: 2, ExpectedUnitPriceRub: 100},
			{ProductID: "aaaaaaaa-0000-0000-0000-000000000001", Quantity: 1, ExpectedUnitPriceRub: 200},
		},
	}
	normalized, hash, err := normalize(first)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.CustomerName != "Test Buyer" || normalized.Phone != "+79000000000" || normalized.Comment != "comment" {
		t.Fatal("contacts were not normalized")
	}
	if first.Items[0].Quantity != 2 || !strings.HasPrefix(first.Items[0].ProductID, "AAAA") {
		t.Fatal("normalization modified caller input")
	}
	second := normalized
	second.Items = []requests.RequestedItem{normalized.Items[1], normalized.Items[0]}
	_, secondHash, err := normalize(second)
	if err != nil || hash != secondHash {
		t.Fatal("equivalent baskets have different fingerprints")
	}
	second.Items[0].ExpectedUnitPriceRub++
	_, differentHash, err := normalize(second)
	if err != nil || hash == differentHash {
		t.Fatal("changed price did not change fingerprint")
	}
}

func TestNormalizeRejectsInvalidSubmission(t *testing.T) {
	valid := func() Submission {
		return Submission{CustomerName: "Test", Phone: "+79000000000", Items: []requests.RequestedItem{
			{ProductID: "aaaaaaaa-0000-0000-0000-000000000001", Quantity: 1, ExpectedUnitPriceRub: 10},
		}}
	}
	cases := map[string]func(*Submission){
		"blank name":        func(s *Submission) { s.CustomerName = " \n " },
		"long unicode name": func(s *Submission) { s.CustomerName = strings.Repeat("я", 101) },
		"NUL":               func(s *Submission) { s.Comment = "\x00" },
		"invalid UTF8":      func(s *Submission) { s.Comment = string([]byte{0xff}) },
		"missing country":   func(s *Submission) { s.Phone = "89000000000" },
		"empty basket":      func(s *Submission) { s.Items = nil },
		"too many items":    func(s *Submission) { s.Items = make([]requests.RequestedItem, MaxItems+1) },
		"bad UUID":          func(s *Submission) { s.Items[0].ProductID = "tire" },
		"zero quantity":     func(s *Submission) { s.Items[0].Quantity = 0 },
		"quantity limit":    func(s *Submission) { s.Items[0].Quantity = MaxQuantity + 1 },
		"negative price":    func(s *Submission) { s.Items[0].ExpectedUnitPriceRub = -1 },
		"duplicate canonical UUID": func(s *Submission) {
			other := s.Items[0]
			other.ProductID = strings.ToUpper(other.ProductID)
			s.Items = append(s.Items, other)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			input := valid()
			mutate(&input)
			if _, _, err := normalize(input); !errors.Is(err, ErrInvalidSubmission) {
				t.Fatalf("got %v, want validation error", err)
			}
		})
	}
}
