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

// TestRestrictedManaFeeWiderKeepsRefusing: the wider fee productions that need
// a kernel model this change does not add stay refused, and the reason names
// that production or the source's second ability, never the plain fee the
// model does carry. Each is a separate follow-up: Cormela's all-at-once
// {U}{B}{R} and Great Hall's Combo Any beside a census ability both lower in
// kgen but diverge from gorge in lockstep (the mana assignment differs), and
// Hydro-Channeler's fee ability sits beside its restricted {U} one.
func TestRestrictedManaFeeWiderKeepsRefusing(t *testing.T) {
	for _, name := range []string{"Cormela, Glamour Thief", "Great Hall of the Citadel", "Hydro-Channeler", "Interplanar Beacon"} {
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
