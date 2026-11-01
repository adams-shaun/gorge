package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newVerbScen is newBLBScen with patch applied to the matchup table before
// the game is set up (so the table's prep sees it).
func newVerbScen(t *testing.T, deck []cardset.Entry, patch func(tbl *Table, id map[string]CardID)) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "blb")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the BLB table", e.Name)
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
	patch(tbl, id)
	pl := id["Plains"]
	var lib [20]CardID
	for i := range lib {
		lib[i] = pl
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
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

// TestOthersDoesntUntap: ReplOthersDoesntUntap (gorge cards.ReplUntap with
// ValidCard$ other than Card.Self): a tapped permanent one replacement
// matches stays tapped unasked; one two match parks the untap step on
// gorge's CR 616.1 order ask (one option per replacement source, in
// battlefield order), keeps it tapped, and the scan resumes after the
// answer; the opponent's matching permanent is not the active player's
// and is untouched; with none matching the permanent untaps.
func TestOthersDoesntUntap(t *testing.T) {
	sc, id := newVerbScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}},
		func(tbl *Table, id map[string]CardID) {
			fc := id["Valley Floodcaller"]
			var f kop.FilterID
			for _, a := range tbl.Cards[fc].Abilities {
				for _, in := range a.Prog {
					if in.Op == kop.OpUntapAll {
						f = in.F // Otters you control
					}
				}
			}
			tbl.Cards[fc].Abilities = append(tbl.Cards[fc].Abilities,
				kop.Ability{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplOthersDoesntUntap, Filter: f}})
		})
	fc, plains := id["Valley Floodcaller"], id["Plains"]
	if !sc.k.gen.untapRepl {
		t.Fatal("prep did not see the replacement")
	}
	a, b := sc.perm(fc, 0), sc.perm(fc, 0)
	land := sc.perm(plains, 0)
	opp := sc.perm(fc, 1)
	for _, o := range []Obj{a, b, land, opp} {
		sc.s.perms[sc.slot(o)].flags |= pfTapped
	}
	sc.s.turn, sc.s.active, sc.s.step, sc.s.ctl = 3, 0, kop.StepUntap, ctlStepBegin
	for i := 0; i < 10 && sc.s.step == kop.StepUntap; i++ {
		if d := sc.s.Step(); d.Kind == DkChoose {
			t.Fatalf("an ask, sub %v: two \"doesn't untap\" effects have nothing to order (CR 502.3, 614.1)", d.Sub)
		}
	}
	if !sc.tapped(a) || !sc.tapped(b) || sc.tapped(land) || !sc.tapped(opp) {
		t.Fatalf("tapped: a %v b %v land %v opp %v; want a, b, opp tapped and the land untapped",
			sc.tapped(a), sc.tapped(b), sc.tapped(land), sc.tapped(opp))
	}
	if sc.s.step == kop.StepUntap {
		t.Fatal("the untap step did not finish")
	}
}
