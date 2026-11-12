package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestImproviseExpend: improvise taps are not mana, so neither the expend
// tally nor CastTotalManaSpent counts them (gorge cast_commit.go: Improvise
// does not spend mana; CountsMana is convoke-only).
func TestImproviseExpend(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Foundry Assembler", Count: 1},
		{Name: "Ornithopter", Count: 2},
		{Name: "Island", Count: 4},
	}
	set := cardsettest.Set(t, "improvise")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.k.gen.anyHist = true
	sc.k.gen.anySpent = true
	sc.lands(0, id["Island"], 3)
	m := sc.hand(id["Foundry Assembler"], 0)
	sc.perm(id["Ornithopter"], 0)
	sc.perm(id["Ornithopter"], 0)
	sc.do(t, AkCast, uint8(m), 0xff)
	d := sc.s.Step()
	for picks := 0; picks < 2; picks++ {
		if d.Kind != DkChoose || d.Sub != ChGenConvoke {
			t.Fatalf("pick %d: decision %v/%d, want the improvise ask", picks, d.Kind, d.Sub)
		}
		var pick Action
		found := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkChoose && a.Source() != 0 && a.Mode() == 6 {
				pick, found = a, true
				break
			}
		}
		if !found {
			t.Fatalf("improvise pick %d not offered: %v", picks, d.Options[:d.N])
		}
		sc.s.Apply(pick)
		d = sc.s.Step()
	}
	if d.Kind != DkPriority {
		t.Fatalf("decision after payment %v, want priority", d.Kind)
	}
	if got := sc.s.g.spent[0]; got != 3 {
		t.Fatalf("expend tally %d, want 3 (the improvise taps are not mana)", got)
	}
	for i := 0; i < int(sc.s.sp); i++ {
		if e := &sc.s.stack[i]; e.kind == seSpell && e.obj == m {
			if e.trigAmt != 3 {
				t.Fatalf("CastTotalManaSpent %d, want 3", e.trigAmt)
			}
			return
		}
	}
	t.Fatalf("Foundry Assembler not on the stack")
}

// TestImproviseLandPip: the coloured pips an artifact cannot pay the untapped
// lands pay (CR 702.126a, 601.2h). Maverick Thopterist is {3}{U}{R}: with two
// artifacts announced the plan pays the credited price ({1}{U}{R} left, the
// pips from the lands); with none the plan stands at the printed cost, and
// both casts pay through the planner (improviseRoute), not the manual window.
// A pin of the shape, not the regression gate: the oracle lockstep
// (oracle.TestImproviseLockstep) is what fails without the routing.
func TestImproviseLandPip(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Maverick Thopterist", Count: 1},
		{Name: "Ornithopter", Count: 2},
		{Name: "Island", Count: 4},
		{Name: "Mountain", Count: 1},
	}
	set := cardsettest.Set(t, "improvise")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	thoptersTapped := func(t *testing.T, sc *scen, o1, o2, isl1, isl2, mtn Obj, spell Obj, spent int) {
		t.Helper()
		for _, o := range []Obj{o1, o2, isl1, isl2, mtn} {
			if sc.s.perms[sc.slot(o)].flags&pfTapped == 0 {
				t.Fatalf("object %d is not tapped: the payment left a source", o)
			}
		}
		if got := sc.s.g.spent[0]; got != uint16(spent) {
			t.Fatalf("expend tally %d, want %d (the improvise taps are not mana)", got, spent)
		}
		for i := 0; i < int(sc.s.sp); i++ {
			if e := &sc.s.stack[i]; e.kind == seSpell && e.obj == spell {
				return
			}
		}
		t.Fatalf("Maverick Thopterist not on the stack")
	}

	// two artifacts announced: the ask poses both, the plan pays the credited
	// price with the lands' pips
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.k.gen.anyHist = true
	sc.k.gen.anySpent = true
	isl1, isl2 := sc.perm(id["Island"], 0), sc.perm(id["Island"], 0)
	mtn := sc.perm(id["Mountain"], 0)
	opt1, opt2 := sc.perm(id["Ornithopter"], 0), sc.perm(id["Ornithopter"], 0)
	m := sc.hand(id["Maverick Thopterist"], 0)
	sc.do(t, AkCast, uint8(m), 0xff)
	d := sc.s.Step()
	for picks := 0; picks < 2; picks++ {
		if d.Kind != DkChoose || d.Sub != ChGenConvoke {
			t.Fatalf("pick %d: decision %v/%d, want the improvise ask", picks, d.Kind, d.Sub)
		}
		var pick Action
		found := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkChoose && a.Source() != 0 && a.Mode() == 6 {
				pick, found = a, true
				break
			}
		}
		if !found {
			t.Fatalf("improvise pick %d not offered: %v", picks, d.Options[:d.N])
		}
		sc.s.Apply(pick)
		d = sc.s.Step()
	}
	if d.Kind != DkPriority {
		t.Fatalf("decision after payment %v, want priority", d.Kind)
	}
	thoptersTapped(t, sc, opt1, opt2, isl1, isl2, mtn, m, 3)

	// no artifact: the plan stands at the printed {3}{U}{R}, paid straight
	// from the lands with no improvise ask
	sc2 := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc2.k.gen.anyHist = true
	sc2.k.gen.anySpent = true
	isl3, isl4 := sc2.perm(id["Island"], 0), sc2.perm(id["Island"], 0)
	isl5, isl6 := sc2.perm(id["Island"], 0), sc2.perm(id["Island"], 0)
	mtn2 := sc2.perm(id["Mountain"], 0)
	m2 := sc2.hand(id["Maverick Thopterist"], 0)
	sc2.do(t, AkCast, uint8(m2), 0xff)
	if d2 := sc2.s.Step(); d2.Kind != DkPriority {
		t.Fatalf("printed cast: decision %v/%d, want priority with no improvise ask", d2.Kind, d2.Sub)
	}
	thoptersTapped(t, sc2, isl3, isl4, isl5, isl6, mtn2, m2, 5)

	// one artifact, three lands: the credited price {2}{U}{R} (4) is more
	// than the sources pay, so gorge's planner declines the shape
	// ("cost:credit") and no cast is offered (the greedy credit is no proof
	// of payable)
	sc3 := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc3.perm(id["Ornithopter"], 0)
	sc3.lands(0, id["Island"], 2)
	sc3.perm(id["Mountain"], 0)
	m3 := sc3.hand(id["Maverick Thopterist"], 0)
	d3 := sc3.s.Step()
	for i := 0; i < int(d3.N); i++ {
		if a := d3.Options[i]; a.Kind() == AkCast && a.Source() == uint8(m3) {
			t.Fatalf("the credited price is more than the credit's sources pay: the cast is still offered")
		}
	}
}
