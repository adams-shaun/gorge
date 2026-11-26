package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newCondSplitScen is a reveal-pool table with a Wren's Run Vanquisher
// (Reveal<1/Elf>:3) twice in hand: seat 0's board prices the {3} part's mana
// from the pool's Forests and a Gemstone Caverns.
func newCondSplitScen(t *testing.T, caverns bool) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Wren's Run Vanquisher", Count: 2},
		{Name: "Gemstone Caverns", Count: 1},
		{Name: "Forest", Count: 5},
	}
	set := cardsettest.Set(t, "reveal")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true, GorgeAltAdd: true})
	if caverns {
		sc.lands(0, id["Forest"], 4)
		sc.perm(id["Gemstone Caverns"], 0)
	} else {
		sc.lands(0, id["Forest"], 5)
	}
	return sc, id
}

// TestAltAddPayableConditionalSplit: the {3} part of a Reveal<1/Elf>:3 cast
// that four Forests cannot fund is still payable when a Gemstone Caverns is
// left to tap: the payment planner's census prices no unit of the
// Condition-split source (gorge defers it, pay source:conditional), so the
// choice does not reverse the cast; the CR 601.2g fallback window activates
// the Caverns and the cast completes (CR 733.1). Five Forests fund the part
// from the pool and complete the same cast with the Caverns absent.
func TestAltAddPayableConditionalSplit(t *testing.T) {
	run := func(t *testing.T, caverns bool) {
		sc, id := newCondSplitScen(t, caverns)
		wrv := sc.hand(id["Wren's Run Vanquisher"], 0)
		elf := sc.hand(id["Wren's Run Vanquisher"], 0) // the reveal part's candidate
		sc.do(t, AkCast, TCode(wrv), scAny)
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 2 || d.Options[0].Target() != 0 || d.Options[1].Target() != 1 {
			t.Fatalf("altadd decision %v, want the reveal part then the {3} part", d.Options[:d.N])
		}
		sc.s.Apply(d.Options[1]) // the {3} part
		tapped0, _ := sc.tappedAndFloating(0)
		for i := 0; i < 8; i++ {
			d = sc.s.Step()
			if d.Kind == DkPriority {
				break
			}
			if d.Kind == DkChoose && d.Sub == ChCostReveal {
				t.Fatalf("the {3} part posed a reveal ask %v", d.Options[:d.N])
			}
			sc.s.Apply(d.Options[0])
		}
		tapped, floating := sc.tappedAndFloating(0)
		if sc.s.objs[wrv].zone != kop.ZoneStack {
			t.Fatalf("the {3} part reversed the cast: zone %d, want the stack (tapped %d->%d, floating %d, elf zone %d)",
				sc.s.objs[wrv].zone, tapped0, tapped, floating, sc.s.objs[elf].zone)
		}
		if floating != 0 {
			t.Fatalf("the paid cast left %d mana floating", floating)
		}
		if tapped != 5 {
			t.Fatalf("%d permanents tapped, want 5 ({1}{G} + {3})", tapped)
		}
		sc.passBoth(t)
		if !sc.onBF(wrv) {
			t.Fatalf("the cast did not resolve (elf zone %d)", sc.s.objs[elf].zone)
		}
	}
	t.Run("four forests and the caverns", func(t *testing.T) { run(t, true) })
	t.Run("five forests", func(t *testing.T) { run(t, false) })
}
