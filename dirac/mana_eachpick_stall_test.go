package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// eachDeferScen is eachReflectScen with the all-at-once {W}{U} source on the
// Firdoch Core instead of the Birds: Judge of Currents (the reflect carrier,
// Mana Flare's body) defers only a Merfolk's tap, and the changeling Core is
// the deck's one Merfolk, so the source is Taps-deferred too.
func eachDeferScen(t *testing.T) (*scen, Slot) {
	t.Helper()
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
		ab.Static.Filter = kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append(append([]kop.Ability(nil), j.Abilities...), ab)
		rewriteMana(tbl, id["Firdoch Core"], func(a *kop.Ability) {
			a.ManaColors, a.ManaEach, a.ManaN = kop.ColW|kop.ColU, true, 1
		})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Judge of Currents"], 0)
	c := sc.perm(id["Firdoch Core"], 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	sl := sc.slot(c)
	if !sc.s.tapsDeferred(sc.k, 0, sl) {
		t.Fatal("precondition: the Core is not Taps-deferred under Judge of Currents")
	}
	if pc := sc.s.manaPCGen(sc.k, sl); pc.manaXv == [kop.NumManaKinds]uint8{} {
		t.Fatal("precondition: the rewritten Core is no all-at-once source")
	}
	return sc, sl
}

// TestEachPickStallsUnpricedReaders: an all-at-once source's reflected pick
// (srcEachPick) is priced only where the census builds and the plan read it;
// floating it unasked stalls in addExtra/addExtraCol. Every other reader of
// the tap's extra (srcExtra) silently drops the pick for the shape, so each
// reader must refuse it itself instead of funding a payment whose decision
// stream would carry a colour ask the kernel cannot pose (the census and plan
// price it, mana_each_reflectpick_test.go).
func TestEachPickStallsUnpricedReaders(t *testing.T) {
	sc, sl := eachDeferScen(t)
	pc := sc.s.manaPCGen(sc.k, sl)
	if r := sc.s.srcEachPick(sc.k, sl, pc); r != 1 {
		t.Fatalf("precondition: srcEachPick = %d, want 1", r)
	}
	// the may-pay-X bound sums srcExtra over every source that can tap
	sc.s.g.stalled = 0
	if n := sc.s.trigXTapExtras(sc.k, 0); sc.s.g.stalled != StallTapExtra {
		t.Fatalf("trigXTapExtras = %d, stalled %d, want StallTapExtra (%d)", n, sc.s.g.stalled, StallTapExtra)
	}
	// a Taps-deferred gap's census unit prices the tap as srcYieldAt + srcExtra
	sc.s.g.stalled = 0
	var a avail
	sc.s.addTapsDeferUnits(sc.k, sl, &a)
	if sc.s.g.stalled != StallTapExtra {
		t.Fatalf("addTapsDeferUnits: stalled %d, want StallTapExtra (%d)", sc.s.g.stalled, StallTapExtra)
	}
	// a tap that would have brought the extra through a path that does not add it
	sc.s.g.stalled = 0
	sc.s.tapExtraStall(sc.k, sl)
	if sc.s.g.stalled != StallTapExtra {
		t.Fatalf("tapExtraStall: stalled %d, want StallTapExtra (%d)", sc.s.g.stalled, StallTapExtra)
	}
}
