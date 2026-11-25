package kgen

import (
	"testing"
)

// TestRestrictedManaFeeWideShape: the spend-restricted mana-fee ability lowers
// for a production wider than one fixed colour when it is the card's only mana
// ability and the production is two mana in any combination of colours
// (Orb of Dragonkind). The kernel plans the two any-colour units directly
// (mana_restrictedsource.go restrSrcAddUnit's rkExt arm), the fee charged from
// the payment's floating mana (CR 605.3b).
func TestRestrictedManaFeeWideShape(t *testing.T) {
	lc := lowerCorpusCard(t, "Orb of Dragonkind")
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
}

// TestRestrictedManaFeeEachShape: Cormela, Glamour Thief's "{1}, {T}: Add
// {U}{B}{R}" (one mana of each of a fixed set of colours, instants and
// sorceries only) lowers as a restricted fee unit: the kernel carries the
// all-at-once production as the unit's first colour beside a fixed rest, and
// the fee is charged from the payment's floating mana (CR 605.3b).
func TestRestrictedManaFeeEachShape(t *testing.T) {
	lc := lowerCorpusCard(t, "Cormela, Glamour Thief")
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
}

// The three wider fee productions that once needed a kernel model now lower:
// Great Hall of the Citadel's Combo Any beside a census ability
// (restrComboWide), Hydro-Channeler's fee ability beside its restricted {U}
// one (restrFeePairOf), and Interplanar Beacon's Combo AnyDifferent pair
// (avail.nDist). Their lowering tests are restrictedmana_feepair_test.go and
// restrictedmana_anydifferent_test.go.
