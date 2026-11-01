package kgen

import (
	"strings"
	"testing"
)

// TestManarestrictBareActivated: the bare `Activated`
// term of RestrictValid$ (gorge pay/payment.go:606-607: every activated
// ability, no card filter) lowers beside Spell.Artifact as a spell filter
// plus a non-zero ManaRestrictAb; Pit Automaton (Activated-only, no spell
// half) is still refused, now by its delayed trigger (its build-stage
// refusal, "has no kop form", stays unreached until that clears).
func TestManarestrictBareActivated(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Guidelight Optimizer", "Purple Dragon Punks", "Pit Automaton"})
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		switch b.def.Name {
		case "Guidelight Optimizer", "Purple Dragon Punks":
			if len(b.refusals) != 0 {
				t.Errorf("%s refused: %v", b.def.Name, b.refusals)
			}
			found := false
			for _, ab := range b.def.Abilities {
				if ab.ManaRestrict != 0 && ab.ManaRestrictAb != 0 {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: no mana ability with ManaRestrict and ManaRestrictAb", b.def.Name)
			}
		}
	}
	for _, c := range lw.cards {
		if c.Name == "Pit Automaton" {
			if len(c.Refusals) == 0 {
				t.Errorf("Pit Automaton (Activated-only restriction, no spell half) was accepted")
			}
			for _, r := range c.Refusals {
				if strings.Contains(r.Reason, "mana restriction \"Activated\"") {
					t.Errorf("Pit Automaton: the bare term must not be the blocker: %v", r)
				}
			}
		}
	}
}
