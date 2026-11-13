package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// beGapScen builds p0's first main phase from a pool fixture's table with the
// named board permanents untapped and the carrier in hand (may false) or in
// exile under a may-play permission (may true). The carrier is a minimal
// appended definition: the lowering reads its cost and BeholdExile flag, not
// its program. sub names the one carrier subtype (for a restricted source's
// admission), "" for none.
func beGapScen(t *testing.T, pool string, cost kop.ManaCost, sub string, may bool, board ...string) (*scen, Obj) {
	t.Helper()
	set := cardsettest.Set(t, pool)
	var deck []cardset.Entry
	seen := map[string]bool{}
	for _, n := range append(board, "Island") {
		if !seen[n] {
			seen[n] = true
			deck = append(deck, cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var subMask kop.SubMask
	if sub != "" {
		found := false
		for i, n := range tbl.Subtypes {
			if n == sub {
				subMask, found = kop.SubMask{}.With(i), true
			}
		}
		if !found {
			t.Fatalf("%q not in the %s table's subtypes", sub, pool)
		}
	}
	carrier := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test BeholdExile", Types: kop.TCreature, Cost: cost, Subtypes: subMask,
		BeholdExile: true, BeholdN: 1,
	})
	if tbl.Exprs == nil {
		tbl.Exprs = []kop.Expr{} // prepareGen builds the generated extension (k.gen non-nil)
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{GorgeLowering: true, NoElide: true})
	for _, n := range board {
		sc.perm(id[n], 0)
	}
	if may {
		o := sc.obj(carrier, 0)
		s := sc.s
		s.objs[o].zone = kop.ZoneExile
		s.g.may[0] = mayPlay{obj: o, ver: s.objs[o].ver, ctrl: 0, until: uint8(kop.DurEOT), turn: mayWord(s.turn)}
		s.g.nmay = 1
		return sc, o
	}
	return sc, sc.hand(carrier, 0)
}

// TestBeholdExileLoweringIsAMayPlayOnly (CR 601.2b, 601.2g): a plain hand
// cast with an additional cost is not one of gorge's potential plays -- its
// planner refuses the shape before it prices the mana -- so no prefix search
// taps a census-gap source for it and dirac must not lower it. A cast
// through a play permission is priced by mayPlayAffordable's retry, which
// does tap the gap, so its lowering stays. Both a tap-another source
// (Springleaf Drum) and a restricted wide source (Flamebraider) are pinned:
// the old gap-kind condition lowered the plain cast for both, and dropping
// the tap-another kind would have stopped lowering the may-play one.
func TestBeholdExileLoweringIsAMayPlayOnly(t *testing.T) {
	for _, c := range []struct {
		name  string
		pool  string
		cost  kop.ManaCost
		sub   string
		board []string
	}{
		{"tap_another", "tapall", kop.ManaCost{Generic: 3, Pips: [6]uint8{0, 1, 0, 0, 0, 0}}, "",
			[]string{"Springleaf Drum", "Coral Merfolk", "Island", "Island", "Forest"}},
		{"restricted_wide", "hybrestr", kop.ManaCost{Generic: 3, Pips: [6]uint8{0, 0, 0, 1, 0, 0}}, "Elemental",
			[]string{"Flamebraider", "Mountain", "Island", "Plains"}},
	} {
		for _, may := range []bool{false, true} {
			sc, o := beGapScen(t, c.pool, c.cost, c.sub, may, c.board...)
			s := sc.s
			if sc.k.gen == nil {
				t.Fatalf("%s mayPlay %v: the generated extension is absent, so the lowering guard is not exercised", c.name, may)
			}
			if !s.scriptSourceLive(sc.k, 0) {
				t.Fatalf("%s mayPlay %v: no live census-gap source on %v", c.name, may, c.board)
			}
			if s.plannerPaysCast(sc.k, 0, s.objs[o].card, o) {
				t.Fatalf("%s mayPlay %v: the base planner already prices the cast, so the fallback is not reached", c.name, may)
			}
			// the scripted prefix search's relaxed proof is what lets the
			// fallback's window count the gap source
			s.g.availX |= availGaps
			cost := s.featCost(sc.k, 0, s.objs[o].card, o, sc.k.cards[s.objs[o].card].cost)
			if !s.affordsFor(sc.k, 0, &cost) {
				t.Fatalf("%s mayPlay %v: the window cannot pay %+v through %v, so the fallback cannot fire", c.name, may, cost, c.board)
			}
			if got := s.gorgeLowers(0, o, may); got != may {
				t.Errorf("%s mayPlay %v: gorgeLowers = %v, want %v (a plain cast has no potential-play prefix search; a may-play one does)", c.name, may, got, may)
			}
		}
	}
}
