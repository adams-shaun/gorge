package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The former findings-doc gaps of the noncombat replacement-order ask and the
// DealDamageFrom player mode: a damage event gorge parks on its CR 616.1 order
// ask that the kernel would deal unasked now stalls loudly, and a departed
// DamageSource$ reads infect from its last known information (CR 113.7a).

// TestParkNCSecondEventStalls: with one event of the op parked on its order
// ask (two Twinflame Tyrants, Burst Lightning at player 1), a second event
// the same two replacements match is not dealt ahead of the first: it stalls
// StallReplPlus.
func TestParkNCSecondEventStalls(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 2}, {Name: "Burst Lightning", Count: 1},
		{Name: "Erudite Wizard", Count: 2}, {Name: "Mountain", Count: 12}}
	sc, id := replScen(t, deck)
	s := sc.s
	sc.perm(id["Twinflame Tyrant"], 0)
	sc.perm(id["Twinflame Tyrant"], 0)
	d := burnToAsk(t, sc, id, uint8(PlayerTarget1))
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder || s.g.nc.n == 0 || s.g.ro.n != 2 {
		t.Fatalf("precondition: want the first event parked on its order ask (nc.n %d ro.n %d)", s.g.nc.n, s.g.ro.n)
	}
	if s.Stalled() {
		t.Fatalf("precondition: stalled %d before the second event", s.StallReason())
	}
	saveNC := s.g.nc
	if s.parkNC(sc.k, 0, NoSlot, id["Burst Lightning"], PlayerTarget1, 2, 0) {
		t.Fatal("a second event parked over the first")
	}
	if s.StallReason() != StallReplPlus {
		t.Fatalf("stall %d after a second competing event, want StallReplPlus (%d)", s.StallReason(), StallReplPlus)
	}
	if s.g.nc != saveNC {
		t.Fatal("the parked event was overwritten")
	}
}

// TestDmgSrcOrderStalls: the DealDamageFrom pass deals through replDamage, so
// an event two Twinflame Tyrants match (gorge's order ask) stalls loudly;
// with one Tyrant there is no ask and nothing stalls.
func TestDmgSrcOrderStalls(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twinflame Tyrant", Count: 2}, {Name: "Burst Lightning", Count: 1},
		{Name: "Erudite Wizard", Count: 2}, {Name: "Mountain", Count: 12}}
	sc, id := replScen(t, deck)
	s := sc.s
	sc.perm(id["Twinflame Tyrant"], 0)
	src := id["Erudite Wizard"]
	if s.dmgSrcOrderStalls(sc.k, 0, NoSlot, src, 0, []TCode{PlayerTarget1}, []int{2}) || s.Stalled() {
		t.Fatalf("one Tyrant: stalled (reason %d), want no ask", s.StallReason())
	}
	sc.perm(id["Twinflame Tyrant"], 0)
	if !s.dmgSrcOrderStalls(sc.k, 0, NoSlot, src, 0, []TCode{PlayerTarget1}, []int{2}) {
		t.Fatal("two Tyrants: the event gorge asks was not reported")
	}
	if s.StallReason() != StallReplPlus {
		t.Fatalf("stall %d, want StallReplPlus (%d)", s.StallReason(), StallReplPlus)
	}
	if s.g.nc.n != 0 || s.g.ro.n != 0 {
		t.Fatalf("the probe left an ask parked (nc.n %d ro.n %d)", s.g.nc.n, s.g.ro.n)
	}
}

// TestLKIInfectSnapshot: a snapshot records infect as the permanent had it
// (printed and not lost), which the departed-source read of dmgSrcPlayers
// uses in place of the printed flag.
func TestLKIInfectSnapshot(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plague Stinger", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Swamp", Count: 12}}
	set := cardsettest.Set(t, "dmgsrcpl")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyInfect {
		t.Fatal("precondition: the table's generated extension has no infect")
	}
	s := sc.s
	stinger, bears := sc.perm(id["Plague Stinger"], 0), sc.perm(id["Grizzly Bears"], 0)
	if !s.infectAt(sc.k, sc.slot(stinger)) {
		t.Fatal("precondition: Plague Stinger has no infect on the battlefield")
	}
	if li := s.snapLKI(stinger); li == 0 || s.lk(li).bits&lkiInfect == 0 {
		t.Fatalf("Plague Stinger's snapshot %d has no infect", li)
	}
	if li := s.snapLKI(bears); li == 0 || s.lk(li).bits&lkiInfect != 0 {
		t.Fatalf("Grizzly Bears' snapshot %d has infect", li)
	}
	s.perms[sc.slot(stinger)].ovr.flags |= ovLoseAll
	if !s.genLost(sc.k, sc.slot(stinger)) {
		t.Fatal("precondition: the Stinger has not lost its abilities")
	}
	if li := s.snapLKI(stinger); li == 0 || s.lk(li).bits&lkiInfect != 0 {
		t.Fatalf("the Stinger's snapshot %d after losing its abilities still has infect", li)
	}
}
