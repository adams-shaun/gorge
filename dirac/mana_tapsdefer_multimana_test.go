package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsDeferMultiManaFixedColourIsAScriptGap: a deferred several-mana tap
// of one fixed colour (Firdoch Core rewritten to "{T}: Add {G}{G}{G}", the
// Llanowar Tribe shape under Cryoshatter) is a script gap whose census unit
// gorge's planner never lists: the offer's census (availFor) counts the Birds
// alone, and the script route floats all three {G} with no colour ask. With the
// several-mana shapes dropped (DebugNoTapsDeferYield) it keeps its ordinary
// census unit, as before.
func TestTapsDeferMultiManaFixedColourIsAScriptGap(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) { a.ManaColors, a.ManaN = kop.ColG, 3 })
	})
	board := func(t *testing.T) (*scen, Obj) {
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		core := sc.perm(id["Firdoch Core"], 0)
		sc.perm(id["Judge of Currents"], 0)
		sc.perm(id["Birds of Paradise"], 0)
		if pc := sc.s.manaPCGen(sc.k, sc.slot(core)); pc.manaCol != kop.ColG || pc.manaN != 3 || !pc.manaTap {
			t.Fatalf("precondition: the Core's mana ability is col %#x n %d tap %v, want a {G}{G}{G} tap", pc.manaCol, pc.manaN, pc.manaTap)
		}
		if !sc.s.tapsDeferred(sc.k, 0, sc.slot(core)) {
			t.Fatal("precondition: the {G}{G}{G} Core is not a deferred source under Judge of Currents")
		}
		return sc, core
	}
	census := func(sc *scen) int {
		var a avail
		sc.s.availFor(sc.k, 0, &a)
		return a.total
	}
	t.Run("the deferred source is a gap, not a census unit", func(t *testing.T) {
		sc, core := board(t)
		if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
			t.Fatal("the three-mana Core is no script gap")
		}
		if got := sc.s.deferGapAnswers(sc.k, sc.slot(core)); got != 1 {
			t.Fatalf("%d colour answers for the {G}{G}{G} gap, want 1", got)
		}
		if n := census(sc); n != 1 {
			t.Fatalf("census total %d, want 1 (the Birds alone)", n)
		}
		before := sc.s.pl[0].pool
		sc.s.tapDeferFloat(sc.k, 0, sc.slot(core), 0, 0)
		want := before
		want[kop.G] += 3
		if sc.s.pl[0].pool != want {
			t.Fatalf("pool %v after the gap's float, want %v (three {G})", sc.s.pl[0].pool, want)
		}
	})
	t.Run("without the several-mana gap the source counts", func(t *testing.T) {
		DebugNoTapsDeferYield = true
		defer func() { DebugNoTapsDeferYield = false }()
		sc, core := board(t)
		if sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
			t.Fatal("precondition: the Core is a script gap with the several-mana gap off")
		}
		if n := census(sc); n != 4 {
			t.Fatalf("census total %d, want 4 (the Birds and the Core's three)", n)
		}
	})
}
