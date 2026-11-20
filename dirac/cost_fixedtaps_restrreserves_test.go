package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The granted-mana prefix search's terminal checks keep a fixed-taps cast's
// candidates apart from the restricted mana too. gorge's planner prices the
// composed cost (cost_fixedtaps.go spellPartDisjoint): a plan whose
// restricted unit taps a candidate the tapX part needs prices the play one
// candidate short, so the play is unpayable and the search must refuse it
// (CR 601.2g-h; std-dsk pair 57, fixedtaps_restr_arm_skips_tap_reservation).
//
// The board is the divergence's: p0's only mana is Enduring Vitality's
// granted creature taps -- Cautious Survivor, Creeping Peeper and Enduring
// Vitality float the {2}{G}, leaving only the summoning-sick Kona for the
// tap-two part. Creeping Peeper's own printed {U} is restricted to
// enchantment spells, and Fear of Exposure is one, so the restricted arm
// could pay the printed cost from it -- by tapping the very permanent the
// part's second candidate must be.
func TestFixedTapsRestrPlanKeepsTheTapCandidatesApart(t *testing.T) {
	sc, id := fixedTapsGrantScen(t)
	k := sc.k
	s := sc.s
	// p0's battlefield: Enduring Vitality#10, Cautious Survivor#5 and
	// Creeping Peeper#8 ready; Bleeding Woods#3, Forest#15 and Island#25
	// tapped; Kona#29 summoning sick; Bleeding Woods#43 controlled by p1.
	sc.perm(id["Enduring Vitality"], 0)
	w3 := sc.perm(id["Bleeding Woods"], 0)
	s.perms[sc.slot(w3)].flags |= pfTapped
	sc.perm(id["Cautious Survivor"], 0)
	sc.perm(id["Creeping Peeper"], 0)
	f15 := sc.perm(id["Forest"], 0)
	s.perms[sc.slot(f15)].flags |= pfTapped
	sc.perm(id["Growing Dread"], 0)
	kona := sc.perm(id["Kona, Rescue Beastie"], 0)
	s.perms[sc.slot(kona)].flags |= pfSick
	sc.perm(id["Bleeding Woods"], 1) // p1's Bleeding Woods#43
	isl := sc.perm(id["Island"], 0)
	s.perms[sc.slot(isl)].flags |= pfTapped
	fe := sc.hand(id["Fear of Exposure"], 0)
	sc.hand(id["Oblivious Bookworm"], 0)

	a := MakeAction(AkCast, TCode(fe), 0, 0)
	cost, _, _, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("the granted-mana search declines Fear of Exposure's cast")
	}
	if cost.MV() != 3 || cost.Pips[kop.G] != 1 {
		t.Fatalf("priced at %+v, want {2}{G}", cost)
	}
	d := s.grantPayDesc(k, a)
	if s.affordsUngrantedPlay(k, 0, a, &cost, NoSlot, d) {
		t.Error("the printed sources price the cast (the board has no untapped printed source that pays it)")
	}
	s2 := s.fork()
	defer s2.unfork()
	var g grantSrch
	if s2.grantSearch(k, 0, &cost, NoSlot, d, s.playNeed(k, a), &g) {
		t.Errorf("the granted-mana search found a prefix for the cast (np %d): a plan priced it by tapping a tapX candidate", g.np)
	}
}

// fixedTapsGrantScen is the divergence board's cards over the std-dsk table.
func fixedTapsGrantScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Fear of Exposure", Count: 1}, {Name: "Enduring Vitality", Count: 1},
		{Name: "Cautious Survivor", Count: 1}, {Name: "Creeping Peeper", Count: 1},
		{Name: "Kona, Rescue Beastie", Count: 1}, {Name: "Oblivious Bookworm", Count: 1},
		{Name: "Bleeding Woods", Count: 2}, {Name: "Forest", Count: 1}, {Name: "Island", Count: 1},
	}
	tbl, da, _, err := cardsettest.Synth(t, "std-dsk").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.cfg |= cfgGorgeAutopay | cfgGorgeUnpriced
	if sc.k.gen == nil || len(sc.k.gen.grantPC) == 0 {
		t.Fatal("precondition: the table has no granted mana ability, so the grant search never runs")
	}
	return sc, id
}

// The control: the same board with one more untapped creature pays the cast
// -- the three grants float the mana and the two spare creatures are the taps
// -- so the refusals above are the reservation's, not the board's.
func TestFixedTapsRestrPlanPaysWithASpareCandidate(t *testing.T) {
	sc, id := fixedTapsGrantScen(t)
	k := sc.k
	s := sc.s
	sc.perm(id["Enduring Vitality"], 0)
	w3 := sc.perm(id["Bleeding Woods"], 0)
	s.perms[sc.slot(w3)].flags |= pfTapped
	sc.perm(id["Cautious Survivor"], 0)
	sc.perm(id["Cautious Survivor"], 0)
	sc.perm(id["Creeping Peeper"], 0)
	sc.perm(id["Kona, Rescue Beastie"], 0) // not sick here: a fifth candidate
	sc.perm(id["Growing Dread"], 0)
	fe := sc.hand(id["Fear of Exposure"], 0)

	a := MakeAction(AkCast, TCode(fe), 0, 0)
	cost, _, _, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("the granted-mana search declines Fear of Exposure's cast")
	}
	d := s.grantPayDesc(k, a)
	s2 := s.fork()
	defer s2.unfork()
	var g grantSrch
	if !s2.grantSearch(k, 0, &cost, NoSlot, d, s.playNeed(k, a), &g) {
		t.Error("the granted-mana search found no prefix for a payable cast")
	}
}
