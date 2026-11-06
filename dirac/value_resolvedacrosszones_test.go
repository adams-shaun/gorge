package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestResolvedThisTurnCountsAcrossZoneChange: Forge's ActivationTable keys
// Count$ResolvedThisTurn by the host card's printed SpellAbility, which a zone
// change does not replace (CR 707.10b: effects that count how many times an
// ability has resolved during the turn treat it as the same ability), so a
// permanent that leaves and returns keeps the turn's tally. South Pole Voyager
// returned by Airbender Ascension therefore does not draw on its second
// resolution since the return when it is already the second overall.
func TestResolvedThisTurnCountsAcrossZoneChange(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Harvestrite Host", Count: 1}, {Name: "Plains", Count: 8}})
	hh := id["Harvestrite Host"]
	ab := histAb(t, sc, hh, kop.TrigETB)
	if sc.k.t.Cards[hh].Abilities[ab].AFlags&kop.AfResolveCount == 0 {
		t.Fatal("Harvestrite Host's trigger does not count its resolutions")
	}
	o := sc.perm(hh, 0)
	resolve := func() int {
		sc.s.res.active = true
		sc.s.res.e = stackEntry{kind: seTrigger, obj: o, srcVer: sc.s.objs[o].ver, card: hh, ab: ab, ctrl: 0}
		if !sc.s.trigResolveOK(sc.k) {
			t.Fatal("the trigger has no intervening-if")
		}
		n := sc.s.resolvedThisTurn(sc.k)
		sc.s.res.active = false
		return n
	}
	if n := resolve(); n != 1 {
		t.Fatalf("the first resolution reads %d, want 1", n)
	}
	// Leaving and returning bumps the object's ver but not its ObjID: the
	// tally must keep counting (the pre-fix key included the ver and read 1).
	sc.s.objs[o].ver++
	if n := resolve(); n != 2 {
		t.Fatalf("the first resolution after a zone change reads %d, want 2", n)
	}
	if n := resolve(); n != 3 {
		t.Fatalf("the second resolution after a zone change reads %d, want 3", n)
	}
}
