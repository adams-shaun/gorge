package kgen

import (
	"strings"
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

// TestRestrictedManaFeeWiderKeepsRefusing: the wider fee productions that need
// a kernel model this change does not add stay refused, and the reason names
// that production or the source's second ability, never the plain fee the
// model does carry. Each is a separate follow-up: Great Hall's Combo Any
// beside a census ability lowers in kgen but diverges from gorge in lockstep
// (the mana assignment differs), and Hydro-Channeler's fee ability sits beside
// its restricted {U} one.
func TestRestrictedManaFeeWiderKeepsRefusing(t *testing.T) {
	for _, name := range []string{"Great Hall of the Citadel", "Hydro-Channeler", "Interplanar Beacon"} {
		t.Run(name, func(t *testing.T) {
			lc := lowerCorpusCard(t, name)
			if len(lc.Refusals) == 0 {
				t.Fatalf("lowered, want a refusal")
			}
			for _, rf := range lc.Refusals {
				if strings.Contains(rf.Reason, "a mana fee") {
					t.Errorf("refusal names the plain fee the model carries: %s", rf.Reason)
				}
			}
		})
	}
}
