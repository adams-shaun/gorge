package kgen

import "testing"

// TestRestrictedManaFeeAnyDifferentShape: Interplanar Beacon's "{1}, {T}: Add
// two mana of different colors. Spend this mana only to cast planeswalker
// spells." beside its plain {T}: Add {C} lowers as a restricted fee unit: the
// kernel plans the distinct pair as two any-colour units the availability
// model couples (mana.go avail.nDist), the fee charged from the payment's
// floating mana (CR 605.3b, 106.1).
func TestRestrictedManaFeeAnyDifferentShape(t *testing.T) {
	lc := lowerCorpusCard(t, "Interplanar Beacon")
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
}
