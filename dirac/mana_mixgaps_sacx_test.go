package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckSacxFiltOwnNode is the kernel-side half of TestSacxFiltOwnCandidateGap
// (dirac_test). A filter gap (Three Tree Mascot) beside a sacrifice-another
// source (Bolg's Company) whose candidates include the play's own {T} source
// (Goblin Tinkerer, a Goblin, entered before the other two). gorge's filter
// search submits the sacrifice's own-source answer and the node it reaches
// ends the line in leftOwnZone (potential_script.go, CR 602.2b): one node of
// the search's budget spent before the other candidates are tried. The
// kernel charges that node without the fork (mana_mixgaps.go otherGap), so
// the found route's node count is one above the branch without the charge.
//
// The found route is the sacrifice's: Goblin Tinkerer's {R} destroy is
// offered only through Bolg's Company (sacrificing Raging Goblin floats
// {B}{R}; the board's Swamp makes no {R}), so the search's route is root
// node, the "which permanent" ask, Tinkerer's own answer (leftOwnZone), then
// the visit that prices the play -- DebugFiltNodeBudget 4 offers it, 3 does
// not. Without the charge the search needs one node less and offers it at 3.
func CheckSacxFiltOwnNode(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	board := func(cfg Config) (*scen, Obj, Obj, Obj, Obj) {
		t.Helper()
		sc := newScenTableLib(t, tbl, id["Swamp"], cfg)
		tink := sc.perm(id["Goblin Tinkerer"], 0)     // the play's own {T} source
		raging := sc.perm(id["Raging Goblin"], 0)     // the found route's sacrifice
		bolg := sc.perm(id["Bolg's Company"], 0)      // the sacrifice-another gap
		mascot := sc.perm(id["Three Tree Mascot"], 0) // the filter gap (a changeling: a candidate too)
		sc.perm(id["Swamp"], 0)
		return sc, tink, raging, bolg, mascot
	}
	playOf := func(sc *scen, o Obj) (Action, bool) {
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == o {
				return a, true
			}
		}
		return 0, false
	}
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}} {
		sc, tink, raging, bolg, mascot := board(cfg)
		if !sc.k.gen.has(id["Three Tree Mascot"], gfFilt) || !sc.s.scriptFilt(sc.k, 0).has(sc.slot(mascot)) {
			t.Fatal("want Three Tree Mascot a filter gap")
		}
		if !sc.k.sacXOf(sc.s.objs[bolg].card) || !sc.s.scriptTapX(sc.k, 0).has(sc.slot(bolg)) {
			t.Fatal("want Bolg's Company a live sacrifice-another gap")
		}
		if !sc.s.mixBoard(sc.k, 0) {
			t.Fatal("want the filter beside the sacrifice-another source a mixed-gap board")
		}
		var cs [MaxPerm]Slot
		if n := sc.s.tapXCands(sc.k, 0, sc.slot(bolg), &cs); n != 3 || cs[0] != sc.slot(tink) || cs[1] != sc.slot(raging) || cs[2] != sc.slot(mascot) {
			t.Fatalf("gorgeLowering=%v: sacrifice candidates %v (%d), want Tinkerer then Raging Goblin then the Mascot (the own source first)", cfg.GorgeLowering, cs[:n], n)
		}
		if _, ok := playOf(sc, tink); !ok {
			t.Fatalf("gorgeLowering=%v: Goblin Tinkerer's {R} destroy not offered: the board never needs the sacrifice", cfg.GorgeLowering)
		}
		// The found route's node count: the play is offered within the
		// pinned budget and not within the one below it, because the search
		// spends Tinkerer's own-source answer's node (leftOwnZone's dead end)
		// before Raging Goblin's route. Without the charge the route is one
		// node shorter and budget 3 offers it: the pin guards the charge.
		for _, c := range []struct {
			budget  int
			offered bool
		}{
			{3, false},
			{4, true},
		} {
			DebugFiltNodeBudget = c.budget
			sc2, tink2, _, _, _ := board(cfg)
			if _, ok := playOf(sc2, tink2); ok != c.offered {
				t.Fatalf("gorgeLowering=%v: budget %d: Goblin Tinkerer's destroy offered=%v, want %v", cfg.GorgeLowering, c.budget, ok, c.offered)
			}
		}
		DebugFiltNodeBudget = 0
	}
}
