package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// eachReflectScen is tapReflectScen over an all-at-once {W}{U} source (the
// Birds rewritten to Azorius Chancery's "{T}: Add {W}{U}") beside a reflect
// carrier (Mana Flare).
func eachReflectScen(t *testing.T) (*scen, Slot) {
	t.Helper()
	sc, sl := tapReflectScen(t, func(a *kop.Ability) { a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1 })
	if pc := sc.s.manaPCGen(sc.k, sl); pc.manaXv == [kop.NumManaKinds]uint8{} {
		t.Fatal("precondition: the rewritten Birds are no all-at-once source")
	}
	return sc, sl
}

// TestEachReflectedPickIsOneOfTheProducedTypes: an all-at-once source under a
// reflect carrier makes its whole production and one more mana of ONE of the
// types it produced (CR 106.12a; gorge's script route answers the carrier's
// colour ask over the produced types), so {W}{U} and a pick are payable, the
// pick of either produced type in turn, and nothing else is.
func TestEachReflectedPickIsOneOfTheProducedTypes(t *testing.T) {
	sc, sl := eachReflectScen(t)
	pc := sc.s.manaPCGen(sc.k, sl)
	if r := sc.s.srcEachPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("srcEachPick = %d, want 1", r)
	}
	sc.s.g.stalled = 0
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if sc.s.g.stalled != 0 {
		t.Fatalf("the census stalled %d", sc.s.g.stalled)
	}
	if a.total != 3 {
		t.Fatalf("precondition: total = %d, want 3 ({W}{U} and the pick)", a.total)
	}
	cost := func(w, u, b, g int) *kop.ManaCost {
		var c kop.ManaCost
		c.Pips[kop.W], c.Pips[kop.U], c.Pips[kop.B], c.Pips[kop.G] = uint8(w), uint8(u), uint8(b), uint8(g)
		return &c
	}
	for _, tc := range []struct {
		name string
		c    *kop.ManaCost
		want bool
	}{
		{"{W}{W}{U}: the pick is W", cost(2, 1, 0, 0), true},
		{"{W}{U}{U}: the pick is U", cost(1, 2, 0, 0), true},
		{"{W}{U}: the production alone", cost(1, 1, 0, 0), true},
		{"{W}{W}{W}: three of one type", cost(3, 0, 0, 0), false},
		{"{U}{U}{U}: three of one type", cost(0, 3, 0, 0), false},
		{"{W}{U}{G}: a type it did not produce", cost(1, 1, 0, 1), false},
		{"{W}{U}{B}: a type it did not produce", cost(1, 1, 1, 0), false},
	} {
		if got := a.affords(tc.c, pipMaskOf(tc.c), 0); got != tc.want {
			t.Errorf("%s: affords = %v, want %v (total=%d)", tc.name, got, tc.want, a.total)
		}
	}
	if a.affords(cost(0, 0, 0, 0), 0, 3) != true {
		t.Errorf("{3} not payable from the three mana")
	}
}
