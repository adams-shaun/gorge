package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrictedManaFeePairShape: Hydro-Channeler's free "{T}: Add {U}" and
// its "{1}, {T}: Add one mana of any color", both restricted to instant and
// sorcery spells, lower as two restricted mana abilities. The kernel joins
// them as one tap with two alternatives (mana_restrictedfeepair.go): the free
// {U} is gorge's census unit, the any-colour fee the cast window's (CR
// 605.3b). The second ability alone is a plain fee at a cost beyond {T}, which
// the refusal table would reject; beside the free one it is the pair.
func TestRestrictedManaFeePairShape(t *testing.T) {
	lc := lowerCorpusCard(t, "Hydro-Channeler")
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	var free, fee *lAbility
	for _, a := range lc.Abilities {
		if a.Kind != int(kop.AbMana) {
			continue
		}
		switch {
		case !a.Mana.Choice && !a.Mana.Each && a.Mana.Colors == "U":
			free = a
		case !a.Mana.Choice && !a.Mana.Each && a.Mana.Colors == "WUBRG":
			fee = a
		}
	}
	if free == nil || fee == nil {
		t.Fatalf("want the free {U} and the any-colour fee mana abilities, got %+v", lc.Abilities)
	}
	for _, c := range []struct {
		name string
		a    *lAbility
	}{{"free", free}, {"fee", fee}} {
		if c.a.Mana.Amount.N != 1 || c.a.Mana.Restrict == nil {
			t.Errorf("%s: want one restricted mana, got %+v", c.name, c.a.Mana)
		}
	}
	if !free.Cost.Tap || free.Cost.Mana.Generic != 0 || free.Cost.Mana.Pips != [5]int{} || free.Cost.Mana.X {
		t.Errorf("free: want a bare {T} cost, got %+v", free.Cost)
	}
	if !fee.Cost.Tap || fee.Cost.Mana.Generic != 1 || fee.Cost.Mana.Pips != [5]int{} {
		t.Errorf("fee: want a {1},{T} cost, got %+v", fee.Cost)
	}
}
