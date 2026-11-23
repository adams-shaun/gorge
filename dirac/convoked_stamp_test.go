package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// convokedScen builds the convoke cast the stamp tests drive: a table whose
// prep holds the convoked reads (one dangling PredProg literal over
// kop.RefConvoked, the shape a lowered Everything Comes to Dust carries), Halo
// Hopper ({3}) in hand, its Llanowar Elves on the battlefield and two Forests
// to pay the rest. It stops at the convoke ask, after the Elves pick, before
// the payment commit: stamp time is finishBuild's stampConvoked.
func convokedScen(t *testing.T) (*scen, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Halo Hopper", Count: 1},
		{Name: "Llanowar Elves", Count: 1},
		{Name: "Forest", Count: 2},
	}
	set := cardsettest.Set(t, "convokefilter")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	// The convoked read the stamp serves: one dangling program literal, the
	// shape a lowered sharesCreatureTypeWith Convoked carrier's table holds.
	// No card references it, so no filter walk ever evaluates it; only the
	// prep scans see it, and they are what sizes the cold pool.
	if len(tbl.FilterProgs) == 0 {
		tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{})
	}
	tbl.FilterProgs = append([]kop.FilterProg(nil), tbl.FilterProgs...)
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	tbl.FilterProgs = append(tbl.FilterProgs, kop.FilterProg{Lits: []kop.FLit{{W: kop.FwSharesCr, R: kop.RefConvoked}}})
	tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredProg, PredN: int16(len(tbl.FilterProgs) - 1)})
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	hopper := sc.hand(id["Halo Hopper"], 0)
	elves := sc.perm(id["Llanowar Elves"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	if !sc.k.gen.uses.has(useConvokedReads) {
		t.Fatal("precondition: the table's convoked literal did not set useConvokedReads")
	}
	sc.do(t, AkCast, TCode(hopper), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("precondition: first decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
	picked := false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkChoose && Obj(a.Source()) == elves && a.Mode() <= 5 {
			sc.s.Apply(a)
			picked = true
			break
		}
	}
	if !picked {
		t.Fatalf("precondition: the Elves' convoke pick not offered: %v", d.Options[:d.N])
	}
	return sc, hopper, elves
}

// TestConvokedStampNamesAPickedCreatureThatLeft (the stamp's zone rule): gorge
// folds the pay-time CastInfo's Convoked IDs into Object.Convoked at commit,
// whatever zone the picked creature is in by then (cast_commit.go's g.Obj
// check passes for an object that moved), so a pick whose creature left the
// battlefield between its pick and the commit still names it. The stamp must
// not filter by zone: convokedOf reads each object where it now is.
func TestConvokedStampNamesAPickedCreatureThatLeft(t *testing.T) {
	sc, _, elves := convokedScen(t)
	sc.s.objs[elves].zone = kop.ZoneGraveyard // it left the battlefield
	sc.s.stampConvoked(sc.k)
	e := sc.s.bld().e
	if e.cvok == 0 {
		t.Fatal("the stamp recorded no convoked set at all")
	}
	got := sc.s.convokedOf(&e)
	if len(got) != 1 || got[0] != elves {
		t.Fatalf("convoked set %v, want just the moved Elves", got)
	}
}

// TestConvokedStampStallsPastSixteenPicks: a seventeenth convoked creature on
// one cast is past the record's width (maxConvoked). The stamp must take the
// pool's capacity contract (capFull, StallConvokedSet), the same stall the
// pool itself takes, not index past the scratch array.
func TestConvokedStampStallsPastSixteenPicks(t *testing.T) {
	sc, _, _ := convokedScen(t)
	b := sc.s.bld()
	for i := 0; i < maxConvoked+1; i++ {
		b.picks[i] = Obj(i + 1)
		b.pickKs[i] = convokePickMask | pickKind(5)
	}
	b.pickN = maxConvoked + 1
	sc.s.stampConvoked(sc.k)
	if sc.s.g.stalled != StallConvokedSet {
		t.Fatalf("stalled %d, want %d: the seventeenth pick must take the capacity contract", sc.s.g.stalled, StallConvokedSet)
	}
	if b.e.cvok != 0 {
		t.Fatalf("stamped cvok %d on a stalled cast", b.e.cvok)
	}
}
