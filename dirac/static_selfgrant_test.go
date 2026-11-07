package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfGrantOverwrittenByLoseAllSetter: Skyknight Squire with three +1/+1
// counters is a Knight by its own static; Witness Protection, attached later,
// makes it lose all abilities and be a Citizen. The Knight grant began in
// layer 4 and outlives the loss (CR 613.6), but the Aura's newer subtype setter
// overwrites it (CR 613.7), as gorge's walk does: a Citizen, not a Knight. Both
// fixed-order readers (hasSubGen and the projection's subtype mask) agree.
func TestSelfGrantOverwrittenByLoseAllSetter(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Skyknight Squire", Count: 1}, {Name: "Plains", Count: 10}}
	tbl, da, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	knight, citizen := slices.Index(tbl.Subtypes, "Knight"), slices.Index(tbl.Subtypes, "Citizen")
	if knight < 0 || citizen < 0 {
		t.Fatalf("matchup subtypes %v lack Knight or Citizen", tbl.Subtypes)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sq := sc.perm(id["Skyknight Squire"], 0)
	sl := sc.slot(sq)
	sc.s.placeCounters(sc.k, sl, int(kop.CtrP1P1), 3, 0, ctrPutEffect)
	sc.s.refreshTy(sc.k)
	if !sc.s.hasSubGen(sc.k, sl, uint8(knight)) {
		t.Fatal("precondition: Skyknight Squire with three counters is not a Knight")
	}
	if sc.k.gen.fxNeed&fxNeedSubSet != 0 || sc.s.walkSlot(sc.k, sl) {
		t.Fatal("precondition: the Squire walks; this test is about the fixed-order path")
	}
	wp := sc.perm(id["Witness Protection"], 1)
	sc.s.attachTo(sc.k, sc.slot(wp), sl)
	sc.s.refreshTy(sc.k)
	if !sc.s.genLost(sc.k, sl) {
		t.Fatal("precondition: Witness Protection did not take the Squire's abilities")
	}
	if !sc.s.hasSubGen(sc.k, sl, uint8(citizen)) || sc.s.hasSubGen(sc.k, sl, uint8(knight)) {
		t.Errorf("hasSubGen: Citizen %v Knight %v; want a Citizen only",
			sc.s.hasSubGen(sc.k, sl, uint8(citizen)), sc.s.hasSubGen(sc.k, sl, uint8(knight)))
	}
	if _, subs, _, _, ok := sc.s.FeatViewTypes(sq); !ok || !subs.Has(citizen) || subs.Has(knight) {
		t.Errorf("projection subtypes %v (changed %v): want Citizen without Knight", subs, ok)
	}
}
