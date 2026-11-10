package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsDeferFixedColourIsAScriptGap: a deferred one-mana tap of one
// fixed colour (Firdoch Core rewritten to "{T}: Add {G}", the Llanowar Elves
// under Cryoshatter shape) is a script gap. gorge's census never lists it, so
// the offer's census (availFor) counts the Birds beside it and not its {G};
// the script route floats exactly its {G}, one answer, no colour ask.
// Without the classification both count.
func TestTapsDeferFixedColourIsAScriptGap(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) { a.ManaColors = kop.ColG })
	})
	board := func(t *testing.T) (*scen, Obj) {
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		core := sc.perm(id["Firdoch Core"], 0)
		sc.perm(id["Judge of Currents"], 0)
		sc.perm(id["Birds of Paradise"], 0)
		if pc := sc.s.manaPCGen(sc.k, sc.slot(core)); pc.manaCol != kop.ColG || pc.manaN != 1 || !pc.manaTap {
			t.Fatalf("precondition: the Core's mana ability is col %#x n %d tap %v, want one {G} tap", pc.manaCol, pc.manaN, pc.manaTap)
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
		if !sc.s.tapsDeferred(sc.k, 0, sc.slot(core)) {
			t.Fatal("precondition: the {G} Core is not a deferred source under Judge of Currents")
		}
		if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
			t.Fatal("the fixed-colour Core is no script gap")
		}
		if got := sc.s.deferGapAnswers(sc.k, sc.slot(core)); got != 1 {
			t.Fatalf("%d colour answers for the {G} gap, want 1", got)
		}
		if n := census(sc); n != 1 {
			t.Fatalf("census total %d, want 1 (the Birds alone)", n)
		}
		before := sc.s.pl[0].pool
		sc.s.tapDeferFloat(sc.k, 0, sc.slot(core), 0)
		want := before
		want[kop.G]++
		if sc.s.pl[0].pool != want {
			t.Fatalf("pool %v after the gap's float, want %v (one {G})", sc.s.pl[0].pool, want)
		}
	})
	t.Run("without the classification the source counts", func(t *testing.T) {
		DebugNoTapsDefer = true
		defer func() { DebugNoTapsDefer = false }()
		sc, core := board(t)
		if sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
			t.Fatal("precondition: the Core is a script gap with the classification off")
		}
		if n := census(sc); n != 2 {
			t.Fatalf("census total %d, want 2 (the Birds and the Core)", n)
		}
	})
}

// TestTapsDeferPainOnlyGapChargesItsDamage: a deferred pain-only fixed-colour
// tap (the Core rewritten to "{T}: Add {G}. It deals 1 damage to you", the
// Elves of Deep Shadow shape under Cryoshatter) is a script gap whose float
// resolves the rider: gorge's prefix activation runs the ability's program,
// so the life total must drop by its damage as the mana is added.
func TestTapsDeferPainOnlyGapChargesItsDamage(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) {
			a.ManaColors = kop.ColG
			a.Prog = []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.Const(1)}}
		})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	core := sc.perm(id["Firdoch Core"], 0)
	sc.perm(id["Judge of Currents"], 0)
	pc := sc.s.manaPCGen(sc.k, sc.slot(core))
	if !pc.painOnly || pc.painN != 1 || pc.manaCol != kop.ColG || !sc.k.painMana {
		t.Fatalf("precondition: painOnly %v painN %d col %#x painMana %v, want a pain-only 1-damage {G} tap", pc.painOnly, pc.painN, pc.manaCol, sc.k.painMana)
	}
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(core)) {
		t.Fatal("precondition: the pain-only Core is no script gap")
	}
	life := sc.s.pl[0].life
	sc.s.tapDeferFloat(sc.k, 0, sc.slot(core), 0)
	if sc.s.pl[0].pool[kop.G] != 1 {
		t.Fatalf("pool %v, want one {G}", sc.s.pl[0].pool)
	}
	if got := sc.s.pl[0].life; got != life-1 {
		t.Fatalf("life %d after the gap's float, want %d (its 1 damage)", got, life-1)
	}
}
