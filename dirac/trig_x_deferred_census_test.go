package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTrigXDeferredCensusCountsTheDeferredSource pins the X announcement's
// one-tap cap against the planner census's deferred-source gap: under Judge
// of Currents, Firdoch Core's tap is deferred, so availFor omits it while the
// potential walk counts it. The cap must read the deferred-inclusive census
// (availForCapped), keeping X = 1 payable instead of narrowing it to 0.
func TestTrigXDeferredCensusCountsTheDeferredSource(t *testing.T) {
	deck := []cardset.Entry{{Name: "Judge of Currents", Count: 1}, {Name: "Firdoch Core", Count: 1}, {Name: "Plains", Count: 38}}
	tbl, da, _, err := cardsettest.Set(t, "tapsdefer").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	core, judge, plains := id["Firdoch Core"], id["Judge of Currents"], id["Plains"]
	// One fixed {G} tap: an any-colour Core prices the potential walk open
	// (unbounded), which skips the cap; the deferred census gap this test
	// pins needs a bounded production.
	ab := &tbl.Cards[core].Abilities[0]
	if ab.ManaColors != kop.ColAny {
		t.Fatalf("precondition: Firdoch Core's mana ability produces %#x, want any colour", ab.ManaColors)
	}
	ab.ManaColors = kop.ColG
	ab.ManaN = 1
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	co := sc.perm(core, 0)
	sc.perm(judge, 0)
	sl := sc.slot(co)
	if !sc.s.tapsDeferred(sc.k, 0, sl) || !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
		t.Fatal("precondition: the Core is not a script-deferred source under Judge of Currents")
	}
	var one avail
	sc.s.availFor(sc.k, 0, &one)
	if one.total != 0 {
		t.Fatalf("precondition: the planner census counts %d mana from the deferred Core, want 0", one.total)
	}
	var pot avail
	if open := sc.s.availPotentialFor(sc.k, 0, &pot, nil, false); open || pot.total != 1 {
		t.Fatalf("precondition: potential walk open=%v total=%d, want false/1", open, pot.total)
	}
	if sc.s.execTrigX(sc.k, 0, 0, trigXCap) {
		t.Fatal("the X announcement suspended")
	}
	if got := sc.s.g.pickLeft; got != 1 {
		t.Fatalf("X bound %d, want 1 (the deferred Core's one tap is payable)", got)
	}
}
