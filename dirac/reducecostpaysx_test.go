package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// reducePaysXScen: two Mocking Sprites (instant and sorcery spells you cast
// cost {1} less) and n Islands under player 0, Mind Spring ({X}{U}{U}) and
// Thought Scour ({U}) in hand.
func reducePaysXScen(t *testing.T, islands int) (sc *scen, spring, scour Obj, isl []Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Mocking Sprite", Count: 2}, {Name: "Mind Spring", Count: 1},
		{Name: "Thought Scour", Count: 1}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "thisturncast")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the thisturncast table", e.Name)
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
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc = newScenTable(t, tbl, Config{NoElide: true})
	for i := 0; i < islands; i++ {
		isl = append(isl, sc.perm(id["Island"], 0))
	}
	sc.perm(id["Mocking Sprite"], 0)
	sc.perm(id["Mocking Sprite"], 0)
	spring = sc.hand(id["Mind Spring"], 0)
	scour = sc.hand(id["Thought Scour"], 0)
	return sc, spring, scour, isl
}

// paid: o is on the stack and the game is back at a priority ask (the
// payment is done).
func (sc *scen) paid(o Obj) bool {
	return sc.s.objs[o].zone == kop.ZoneStack && sc.s.ctl == ctlPriority
}

func (sc *scen) untapped(os []Obj) (n int) {
	for _, o := range os {
		if !sc.tapped(o) {
			n++
		}
	}
	return n
}

// TestReduceCostPaysX: a reduction larger than the printed generic part
// pays into {X} (CR 601.2f; gorge's xAsk prices X=x, then the composed
// reductions). Mind Spring is {X}{U}{U}; two Mocking Sprites take {2} off,
// so five Islands (UU + 3 spare) fund X up to 5 (UU + X-2 = 5 mana for
// X=5), not the 3 that the cost floored to {U}{U} before X offers.
func TestReduceCostPaysX(t *testing.T) {
	sc, spring, _, isl := reducePaysXScen(t, 5)
	sc.do(t, AkCast, uint8(spring), 0xff)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	if d.N != 6 {
		t.Fatalf("X ask offers %d values, want 0..5", d.N)
	}
	for x := 0; x < int(d.N); x++ {
		if d.Options[x].Mode() != uint8(x) {
			t.Fatalf("option %d is X=%d", x, d.Options[x].Mode())
		}
	}
	// the chosen X is charged with the reduction composed: X=5 is five mana
	sc.s.Apply(d.Options[5])
	for i := 0; i < 20 && !sc.paid(spring); i++ {
		d = sc.s.Step()
		if d.Kind == DkChoose || d.Kind == DkTarget {
			t.Fatalf("unexpected decision kind %d sub %d paying X=5", d.Kind, d.Sub)
		}
		if d.N > 0 {
			sc.s.Apply(d.Options[0])
		}
	}
	if sc.s.objs[spring].zone != kop.ZoneStack {
		t.Fatalf("Mind Spring zone %d, want the stack", sc.s.objs[spring].zone)
	}
	if n := sc.untapped(isl); n != 0 {
		t.Fatalf("%d Islands untapped after X=5 with a {2} reduction, want 0", n)
	}

	// a smaller X leaves the rest: X=2 is UU only after the reduction
	sc, spring, _, isl = reducePaysXScen(t, 5)
	sc.do(t, AkCast, uint8(spring), 0xff)
	d = sc.s.Step()
	sc.s.Apply(d.Options[2])
	for i := 0; i < 20 && !sc.paid(spring); i++ {
		d = sc.s.Step()
		if d.N > 0 {
			sc.s.Apply(d.Options[0])
		}
	}
	if n := sc.untapped(isl); n != 3 {
		t.Fatalf("%d Islands untapped after X=2 with a {2} reduction, want 3", n)
	}

	// fewer lands than the pips: the ask is not offered
	sc, spring, _, _ = reducePaysXScen(t, 1)
	d = sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(spring)) {
			t.Fatalf("Mind Spring offered with one Island")
		}
	}
}

// TestReduceCostNonXUnchanged: the reduction of a spell without {X} is
// floored at zero as before (Thought Scour's {U} has no generic part), so
// two Sprites charge one Island.
func TestReduceCostNonXUnchanged(t *testing.T) {
	sc, _, scour, isl := reducePaysXScen(t, 5)
	sc.do(t, AkCast, uint8(scour), 0xff)
	for i := 0; i < 20 && !sc.paid(scour); i++ {
		d := sc.s.Step()
		if d.N > 0 {
			sc.s.Apply(d.Options[0])
		}
	}
	if sc.s.objs[scour].zone != kop.ZoneStack {
		t.Fatalf("Thought Scour zone %d, want the stack", sc.s.objs[scour].zone)
	}
	if n := sc.untapped(isl); n != 4 {
		t.Fatalf("%d Islands untapped after Thought Scour, want 4", n)
	}
}
