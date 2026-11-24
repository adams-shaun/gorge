package kgen

import "testing"

// TestRestrictedManaFeePairShape: Hydro-Channeler's "{1}, {T}: Add one mana of
// any color" beside its free "{T}: Add {U}", both spendable only on instant
// and sorcery spells, lowers as one restricted source of two alternatives:
// the free {U} is the census unit, the any-colour fee alternative the cast
// window's (CR 605.3b), one tap between them.
func TestRestrictedManaFeePairShape(t *testing.T) {
	lc := lowerCorpusCard(t, "Hydro-Channeler")
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
}
