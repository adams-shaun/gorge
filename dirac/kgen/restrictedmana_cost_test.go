package kgen

import (
	"testing"
)

// TestRestrictedManaCostUnit: a spend-restricted mana ability whose cost is
// generic or coloured mana plus {T} (Castle Garenbrig's "{2}{G}{G}, {T}: Add six
// {G}", creature spells and creature abilities only) lowers: gorge's
// cast-window probe prices such a source like an unrestricted one, the fee
// paid from the floating mana before the production joins (engine/rules
// cast_window.go castWindowGenericCostShape), and the kernel plans it as a
// restricted unit of its own (dirac mana_restrictedsource.go) with the fee on
// the view. A cost part beyond the mana fee (a counter cost, Crucible of the
// Spirit Dragon) still refuses by naming that part.
func TestRestrictedManaCostUnit(t *testing.T) {
	lc := lowerCorpusCard(t, "Castle Garenbrig")
	for _, rf := range lc.Refusals {
		t.Errorf("Castle Garenbrig: refused: %s", rf.Reason)
	}
	var got *lAbility
	for _, ab := range lc.Abilities {
		if ab.Kind == akMana && ab.Cost.Tap && !ab.Cost.Mana.None &&
			(ab.Cost.Mana.Generic != 0 || ab.Cost.Mana.Pips != [5]int{} || ab.Cost.Mana.C != 0) &&
			ab.Mana != nil && ab.Mana.Restrict != nil {
			got = ab
		}
	}
	if got == nil {
		t.Fatalf("no lowered restricted mana-fee ability among %d", len(lc.Abilities))
	}
	if got.Cost.Mana.Generic != 2 {
		t.Errorf("fee generic = %d, want 2", got.Cost.Mana.Generic)
	}
	if got.Cost.Mana.Pips != [5]int{0, 0, 0, 0, 2} {
		t.Errorf("fee pips = %v, want {G}{G} (the fee is {2}{G}{G})", got.Cost.Mana.Pips)
	}
	if got.Mana.Colors != "G" || !got.Mana.Amount.isConst() || got.Mana.Amount.N != 6 {
		t.Errorf("production = %q amount %+v, want six {G}", got.Mana.Colors, got.Mana.Amount)
	}
}

// TestRestrictedManaCostCounterKeepsRefusing: a restriction on a source whose
// cost goes beyond {T} into a counter cost still refuses by naming that part.
func TestRestrictedManaCostCounterKeepsRefusing(t *testing.T) {
	for _, name := range []string{"Crucible of the Spirit Dragon", "Throne of Makindi"} {
		lc := lowerCorpusCard(t, name)
		if len(lc.Refusals) == 0 {
			t.Errorf("%s: lowered, want a refusal", name)
		}
	}
}
