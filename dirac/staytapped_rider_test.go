package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// stayTappedLandBoard is the manarider table with a hand-built land: a plain
// {T} ability beside a {T} ability carrying the stay-tapped rider.
func stayTappedLandBoard(t *testing.T) (*scen, CardID) {
	t.Helper()
	tbl, err := cardsettest.Set(t, "manarider").Pool()
	if err != nil {
		t.Fatal(err)
	}
	land := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Stay-Tapped Land",
		Types: kop.TLand,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColB | kop.ColR, ManaN: 1,
				AFlags: kop.AfManaChoice, ManaRider: kop.ManaRiderStayTapped,
				Prog: []kop.Instr{{Op: kop.OpSkipUntap, Obj: kop.RefSelf}}},
		},
	})
	return newScenTable(t, tbl, Config{NoElide: true}), land
}

// TestStayTappedRiderSkipsTheNextUntap: a mana ability carrying the
// stay-tapped rider (Cinder Marsh's A2) keeps its source tapped through its
// controller's next untap step and the one after untaps it. The table is a
// hand-built land so the rider, not a printed card, is what the untap step
// reads.
func TestStayTappedRiderSkipsTheNextUntap(t *testing.T) {
	sc, land := stayTappedLandBoard(t)
	o := sc.perm(land, 0)
	sl := sc.slot(o)
	if !sc.s.riderPlainFirst(sc.k, sl) {
		t.Fatal("precondition: the hand-built land is not a plain first ability beside a rider")
	}
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok || a.ManaRider != kop.ManaRiderStayTapped {
		t.Fatalf("precondition: rider ability %v rider %d, want ManaRiderStayTapped", ok, a.ManaRider)
	}
	if !sc.s.riderGap(sc.k, 0, sl, 0) { // the {B} answer
		t.Fatal("activation refused")
	}
	if !sc.tapped(o) {
		t.Fatal("the activation did not tap the land")
	}
	if got := sc.s.pl[0].pool[kop.B]; got != 1 {
		t.Fatalf("{B} in pool %d, want 1", got)
	}
	untap := func() {
		sc.s.active, sc.s.step = 0, kop.StepUntap
		sc.s.beginStep(sc.k)
	}
	untap()
	if !sc.tapped(o) {
		t.Fatal("the controller's next untap step untapped it")
	}
	untap()
	if sc.tapped(o) {
		t.Fatal("the untap step after did not untap it")
	}
}

// TestStayTappedRiderMarkDiesWithTheSource: the DBStayTapped Effect's
// ExileOnMoved$ Battlefield link is free. The mark rides the battlefield
// permanent's record, so a land that leaves and returns is a new object with
// no mark and untaps in its next untap step.
func TestStayTappedRiderMarkDiesWithTheSource(t *testing.T) {
	sc, land := stayTappedLandBoard(t)
	o := sc.perm(land, 0)
	sl := sc.slot(o)
	ver := sc.s.objs[o].ver
	if !sc.s.riderGap(sc.k, 0, sl, 0) {
		t.Fatal("activation refused")
	}
	if !sc.tapped(o) {
		t.Fatal("the activation did not tap the land")
	}
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	sc.s.enter(sc.k, o, 0, false, kop.ZoneGraveyard)
	sc.s.npend = 0
	if sc.s.objs[o].ver == ver {
		t.Fatalf("precondition: version %d before and after the blink, want a new object", ver)
	}
	if zone := sc.s.objs[o].zone; zone != kop.ZoneBattlefield {
		t.Fatalf("precondition: zone %d after the blink, want the battlefield", zone)
	}
	sc.s.perms[sc.slot(o)].flags |= pfTapped
	sc.s.active, sc.s.step = 0, kop.StepUntap
	sc.s.beginStep(sc.k)
	if sc.tapped(o) {
		t.Fatal("the next untap step kept the returned land tapped: the mark survived the move")
	}
}
