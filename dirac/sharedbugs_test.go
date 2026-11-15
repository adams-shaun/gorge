package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sharedBugScen is a scenario on the sharedbugs pool, Forests in the library.
func sharedBugScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Forest", Count: 10}}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := cardsettest.Set(t, "sharedbugs").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true}), id
}

// A card that left the graveyard and came back is a new object nobody discarded.
func TestMayhemFollowsTheDiscardedObject(t *testing.T) {
	sc, id := sharedBugScen(t, "Electro's Bolt")
	s := sc.s
	o := sc.hand(id["Electro's Bolt"], 0)
	s.discardObj(sc.k, o)
	if !s.mObj(sc.k, mechMayhem, o) {
		t.Fatal("not marked as it was discarded")
	}
	s.detach(o)
	s.placeFromK(sc.k, o, kop.ZoneExile, false, kop.ZoneGraveyard, false)
	s.detach(o)
	s.placeFromK(sc.k, o, kop.ZoneGraveyard, false, kop.ZoneExile, false)
	if s.mObj(sc.k, mechMayhem, o) {
		t.Fatal("still marked after it moved")
	}
}

// CR 701.60a: a change of controller leaves the designation.
func TestSuspectedStaysThroughAControlChange(t *testing.T) {
	sc, id := sharedBugScen(t, "Barbed Servitor")
	s := sc.s
	sl := sc.slot(sc.perm(id["Barbed Servitor"], 0))
	s.mSlSet(sc.k, mechSuspected, sl)
	if !s.gainControl(sl, 1, 1, kop.DurEOT, NoObj, 0, false) {
		t.Fatal("no control change")
	}
	if !s.suspected(sc.k, sl) {
		t.Fatal("the designation ended with the control change")
	}
}

// CR 608.2h: a departed creature keeps what was spent to cast it.
func TestSpendOfADepartedSource(t *testing.T) {
	sc, id := sharedBugScen(t, "Astelli Reclaimer")
	s := sc.s
	c := id["Astelli Reclaimer"]
	ab := -1
	for i := range sc.k.t.Cards[c].Abilities {
		if a := &sc.k.t.Cards[c].Abilities[i]; a.Kind == kop.AbTriggered && a.Trig.Flags2&kop.Tf2SpentBound != 0 {
			ab = i
		}
	}
	if ab < 0 {
		t.Fatal("no spend-bound trigger")
	}
	o := sc.perm(c, 0)
	e := stackEntry{kind: seTrigger, obj: o, srcVer: s.objs[o].ver, card: c, ab: uint8(ab), x: 3, trigObj: NoObj}
	x := &kop.Expr{Kind: kop.ExCastManaSpent}
	if got := s.exprCold(sc.k, x, s.ctxEntry(&e)); got != 3 {
		t.Fatalf("on the battlefield: %d, want 3", got)
	}
	s.leave(sc.k, sc.slot(o), kop.ZoneGraveyard, false)
	if got := s.exprCold(sc.k, x, s.ctxEntry(&e)); got != 3 {
		t.Fatalf("after it left: %d, want the 3 it left with", got)
	}
}

// CR 702.166d: a bargained Brave the Wilds chooses its target as it is cast.
func TestBargainedSpellChoosesItsTarget(t *testing.T) {
	sc, id := sharedBugScen(t, "Brave the Wilds", "Ornithopter")
	s := sc.s
	spell := sc.hand(id["Brave the Wilds"], 0)
	sc.perm(id["Ornithopter"], 0)
	land := sc.perm(id["Forest"], 0)
	d := s.Step()
	a := MakeAction(AkCastCasualty, TCode(spell), 0, 0)
	if !hasOption(&d, a) {
		t.Fatalf("no bargained cast in %v", d.Options[:d.N])
	}
	s.Apply(a)
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenBargain {
		t.Fatalf("after the cast: kind %d sub %d, want the bargain ask", d.Kind, d.Sub)
	}
	s.Apply(d.Options[0])
	d = s.Step()
	if d.Kind != DkTarget || !hasOption(&d, MakeAction(AkTarget, 0, TCode(land), 0)) {
		t.Fatalf("after the bargain: kind %d options %v, want the land target", d.Kind, d.Options[:d.N])
	}
}

// CR 601.2f: Blaze with RRRR and three Goblin Electromancers can announce X = 6.
func TestXCeilingUnderReducers(t *testing.T) {
	sc, id := sharedBugScen(t, "Blaze", "Goblin Electromancer", "Mountain")
	s := sc.s
	for i := 0; i < 3; i++ {
		sc.perm(id["Goblin Electromancer"], 0)
	}
	for i := 0; i < 4; i++ {
		sc.perm(id["Mountain"], 0)
	}
	blaze := sc.hand(id["Blaze"], 0)
	d := s.Step()
	s.Apply(MakeAction(AkCast, TCode(blaze), 0, 0))
	d = s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	if !hasOption(&d, MakeAction(AkChoose, 0, 0, 6)) || hasOption(&d, MakeAction(AkChoose, 0, 0, 7)) {
		t.Fatalf("X ask options %v, want 0..6", d.Options[:d.N])
	}
}
