package catalogdata

import "testing"

func TestDetailsRejectWrongSubtype(t *testing.T) {
	for _, d := range []Details{{Kind: "tyre", PCD: "10x335", UnitsPerSale: 1}, {Kind: "wheel", Season: "summer", UnitsPerSale: 1}} {
		if d.Validate() == nil {
			t.Fatal("incompatible properties accepted")
		}
	}
}
