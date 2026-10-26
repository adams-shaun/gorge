package chars

import "testing"

// TestKindredIsACardType pins CR 205.2a (kindred is a card type) and CR 308.3
// (tribal is its retired name) in the shared chars vocabulary, so the
// card-type and creature-subtype reads cannot disagree.
func TestKindredIsACardType(t *testing.T) {
	if !IsCardType("Kindred") {
		t.Fatal(`IsCardType("Kindred") = false, want true (CR 205.2a)`)
	}
	if IsCreatureSubtype("Kindred") {
		t.Fatal(`IsCreatureSubtype("Kindred") = true, want false (a card type is not a subtype)`)
	}
	if !IsCardType("Tribal") {
		t.Fatal(`IsCardType("Tribal") = false, want true (CR 308.3's retired name)`)
	}
}
