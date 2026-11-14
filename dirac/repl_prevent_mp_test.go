//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fourSeatScen is newScenTableLib at n = MaxSeats: a four-seat game over the
// pool pool with each seat's library 20 copies of land and deck's cards in
// the table, hands back in the libraries, at seat 0's main phase 1
// (scenario_test.go is not mp-tagged).
func fourSeatScen(t *testing.T, pool, land string, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set, err := cardset.ByName(pool)
	if err != nil {
		t.Skip(err)
	}
	set = cardset.AllowUnverified(set)
	decks := make([][]cardset.Entry, MaxSeats)
	for p := range decks {
		decks[p] = deck
	}
	tbl, da, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for p := range da {
		for _, c := range da[p] {
			id[tbl.Cards[c].Name] = c
		}
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id[land]
	}
	s := new(State)
	s.InitN(tbl, [MaxSeats][]CardID{lib[:], lib[:], lib[:], lib[:]}, MaxSeats, 1, Config{NoElide: true})
	for p := uint8(0); p < MaxSeats; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// TestReplDamageFourSeat: seats 2 and 3 are real controllers in the
// four-seat class, and the damage they deal takes the kernel's damage
// replacements like any other seat's. The printed K:Prevent keyword and the
// prevent-to-self replacement (ReplPreventDamageToSelf in gorge: Istvan,
// Guardian Naga) lower to the same CardDef.PreventDmgTo fact
// (dirac/kgen/lower_preventkw.go), so one carrier covers both: Dawn
// Elemental, "Prevent all damage that would be dealt to CARDNAME."
func TestReplDamageFourSeat(t *testing.T) {
	sc, id := fourSeatScen(t, "preventkw", "Plains", []cardset.Entry{
		{Name: "Dawn Elemental", Count: 1}, {Name: "Hill Giant", Count: 2},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Plains", Count: 8},
	})
	dawn := sc.perm(id["Dawn Elemental"], 0) // the prevention carrier, seat 0's
	bears := sc.perm(id["Grizzly Bears"], 1) // no prevention: the control
	g2 := sc.perm(id["Hill Giant"], 2)       // seat 2's creature: a real controller
	g3 := sc.perm(id["Hill Giant"], 3)       // seat 3's
	shock := id["Shock"]
	if sc.k.gen == nil || sc.k.gen.anyRepl&rbPrevDmgTo == 0 ||
		sc.k.gen.replBits[id["Dawn Elemental"]]&rbPrevDmgTo == 0 {
		t.Fatal("no printed prevent-all-damage fact on Dawn Elemental in the preventkw table")
	}
	// Noncombat burn from seat 2 and from seat 3: prevented whole on the
	// carrier, and lands on a permanent without the fact, so the prevention
	// is the carrier's, not a blanket stop.
	if n := sc.s.replDamage(sc.k, 2, NoSlot, shock, TCode(dawn), false, 3); n != 0 {
		t.Fatalf("seat 2's Shock dealt %d to Dawn Elemental, want 0 (prevented)", n)
	}
	if n := sc.s.replDamage(sc.k, 3, NoSlot, shock, TCode(dawn), false, 2); n != 0 {
		t.Fatalf("seat 3's Shock dealt %d to Dawn Elemental, want 0 (prevented)", n)
	}
	if n := sc.s.replDamage(sc.k, 2, NoSlot, shock, TCode(bears), false, 3); n != 3 {
		t.Fatalf("seat 2's Shock dealt %d to Grizzly Bears, want 3 (no prevention)", n)
	}
	// Combat damage from seat 2's creature, through the combat pass the way
	// combat.go runs it: prevented, and the pass to the control still dealt.
	r := &dmgRec{}
	r.ev[0] = dmgEv{src: sc.slot(g2), dst: sc.slot(dawn), n: 3}
	r.nev = 1
	r.dealt[sc.slot(g2)] += 3
	r.toPerm[sc.slot(dawn)] += 3
	sc.s.replCombatGen(sc.k, r)
	if r.toPerm[sc.slot(dawn)] != 0 {
		t.Fatalf("seat 2's combat damage left %d on Dawn Elemental, want 0 (prevented)", r.toPerm[sc.slot(dawn)])
	}
	r = &dmgRec{}
	r.ev[0] = dmgEv{src: sc.slot(g3), dst: sc.slot(bears), n: 3}
	r.nev = 1
	r.dealt[sc.slot(g3)] += 3
	r.toPerm[sc.slot(bears)] += 3
	sc.s.replCombatGen(sc.k, r)
	if r.toPerm[sc.slot(bears)] != 3 {
		t.Fatalf("seat 3's combat damage left %d on Grizzly Bears, want 3 (no prevention)", r.toPerm[sc.slot(bears)])
	}

	// The filtered doubler (Gratuitous Violence: a creature its controller
	// controls, to a permanent or player) reads a seat 2 source too.
	sc2, id2 := fourSeatScen(t, "replmult", "Mountain", []cardset.Entry{
		{Name: "Gratuitous Violence", Count: 1}, {Name: "Elite Vanguard", Count: 2},
		{Name: "Shock", Count: 1}, {Name: "Mountain", Count: 8},
	})
	sc2.perm(id2["Gratuitous Violence"], 2)
	src := sc2.perm(id2["Elite Vanguard"], 2) // in the filter
	rcp := sc2.perm(id2["Elite Vanguard"], 0) // a permanent an opponent of seat 2 controls
	if sc2.k.gen == nil || sc2.k.gen.anyFiltRepl == false {
		t.Fatal("the replmult table has no filtered doubler: dblDmgN never runs")
	}
	if sc2.k.gen.anyRepl&(rbPlusDmg|rbPrevNC|rbShield|rbSelfDmg|rbPrevCol|rbPrevDmgTo) != 0 {
		t.Fatal("the replmult scenario also carries other damage replacements: the doubling reading is not pinned")
	}
	if n := sc2.s.dblDmgN(sc2.k, 2, sc2.slot(src), id2["Elite Vanguard"], 0, sc2.slot(rcp), false); n != 1 {
		t.Fatalf("dblDmgN for seat 2's source = %d, want 1 (seat 2's Violence)", n)
	}
	if n := sc2.s.replDamage(sc2.k, 2, sc2.slot(src), id2["Elite Vanguard"], TCode(rcp), false, 1); n != 2 {
		t.Fatalf("seat 2's Vanguard dealt %d, want 2 (doubled by seat 2's Violence)", n)
	}
	// A source outside the filter takes no doubling.
	if n := sc2.s.replDamage(sc2.k, 2, NoSlot, id2["Shock"], TCode(rcp), false, 2); n != 2 {
		t.Fatalf("seat 2's Shock dealt %d, want 2 (no creature source: no doubling)", n)
	}
	// A controller beyond the game still matches no doubler.
	if n := sc2.s.dblDmgN(sc2.k, MaxSeats, NoSlot, id2["Elite Vanguard"], 0, NoSlot, false); n != 0 {
		t.Fatalf("dblDmgN with no controller = %d, want 0", n)
	}
}
