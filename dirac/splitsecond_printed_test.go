package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// printedSplitSecondScen builds the freeplay matchup with Opt carrying a
// printed split second (CardDef.SplitSecond, as kgen lowers Sudden Shock's
// K:Split second; no built set holds a split second card) when flag is set.
func printedSplitSecondScen(t *testing.T, flag bool) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Island", Count: 12}, {Name: "Mountain", Count: 8}, {Name: "Opt", Count: 2}, {Name: "Shock", Count: 2}}
	set := cardsettest.Set(t, "freeplay")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not supported in the freeplay table", e.Name)
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
	tbl.Cards[id["Opt"]].SplitSecond = flag
	return newScenTableLib(t, tbl, id["Island"], Config{NoElide: true}), id
}

// castsOffered counts the options of the next decision that split second
// bars (casts and non-mana activations), and returns the decision.
func (sc *scen) castsOffered() (n int, d Decision) {
	d = sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if sc.s.splitSecondBars(sc.k, d.Options[i]) {
			n++
		}
	}
	return n, d
}

// TestPrintedSplitSecondBarsResponses: with a spell carrying a printed split
// second on the stack (CR 702.61a), neither player is offered a cast; a copy
// of that spell (CR 707.2) bars them too. Without the printed keyword the
// opponent is offered Shock in response, as gorge offers it.
func TestPrintedSplitSecondBarsResponses(t *testing.T) {
	for _, flag := range []bool{false, true} {
		sc, id := printedSplitSecondScen(t, flag)
		if sc.k.gen == nil || sc.k.gen.anySplitSecond != flag {
			t.Fatalf("flag %v: genPrep.anySplitSecond = %v", flag, sc.k.gen != nil && sc.k.gen.anySplitSecond)
		}
		sc.lands(0, id["Island"], 2)
		sc.lands(1, id["Mountain"], 2)
		opt := sc.hand(id["Opt"], 0)
		sc.hand(id["Opt"], 0)
		shock := sc.hand(id["Shock"], 1)
		sc.castCard(t, opt)
		for i := 0; i < 8; i++ { // the cast's payment picks, up to seat 0's priority
			if d := sc.s.Step(); d.Kind != DkPriority {
				sc.s.Apply(d.Options[0])
			}
		}
		if sc.s.sp != 1 || sc.s.stack[0].obj != opt {
			t.Fatalf("flag %v: Opt is not the one spell on the stack (sp %d)", flag, sc.s.sp)
		}
		n, d := sc.castsOffered()
		if d.Kind != DkPriority || d.Player != 0 {
			t.Fatalf("flag %v: decision kind %d player %d, want seat 0's priority", flag, d.Kind, d.Player)
		}
		if flag != (n == 0) {
			t.Errorf("flag %v: seat 0 is offered %d casts or activations with Opt on the stack", flag, n)
		}
		sc.do(t, AkPass, scAny, scAny)
		n, d = sc.castsOffered()
		if d.Kind != DkPriority || d.Player != 1 {
			t.Fatalf("flag %v: decision kind %d player %d, want seat 1's priority", flag, d.Kind, d.Player)
		}
		shockOffered := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(shock)) {
				shockOffered = true
			}
		}
		if flag == shockOffered || flag != (n == 0) {
			t.Errorf("flag %v: Shock offered %v, %d casts or activations offered", flag, shockOffered, n)
		}
		// the spell on the stack becomes a copy of itself: it keeps the
		// printed keyword (CR 707.2)
		sc.s.objs[opt].of |= ofCopy
		if got := sc.s.splitSecondHolds(sc.k); got != flag {
			t.Errorf("flag %v: a copy of Opt on the stack holds split second = %v", flag, got)
		}
		if n, _ = sc.castsOffered(); flag != (n == 0) {
			t.Errorf("flag %v: seat 1 is offered %d casts or activations with a copy of Opt on the stack", flag, n)
		}
	}
}
