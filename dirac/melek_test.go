package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestMelekOnlyFirstSpellValidSpell: Melek's OnlyFirstSpell$ ReduceCost
// gates the first instant or sorcery of the turn through ValidSpell$
// the {3} applies to the first Inspiration, never
// to a creature, an earlier creature cast does not spend the discount and
// an earlier instant does.
func TestMelekOnlyFirstSpellValidSpell(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Melek, Reforged Researcher", Count: 4},
		{Name: "Inspiration", Count: 8},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 12},
		{Name: "Mountain", Count: 4},
		{Name: "Forest", Count: 4},
	}
	set := cardsettest.Set(t, "reducegates")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	if !sc.k.gen.castRingOn {
		t.Fatal("the reducegates table does not turn the cast ring on")
	}
	sc.perm(id["Melek, Reforged Researcher"], 0)
	insp := sc.hand(id["Inspiration"], 0)
	bear := sc.hand(id["Grizzly Bears"], 0)
	full := sc.k.t.Cards[id["Inspiration"]].Cost
	if full.Generic != 3 {
		t.Fatalf("Inspiration printed generic %d, want 3", full.Generic)
	}
	if g := sc.s.adjustCost(sc.k, 0, id["Inspiration"], NoObj, full).Generic; g != 0 {
		t.Fatalf("first Inspiration at the offer: generic %d, want 0 (Melek's {3})", g)
	}
	if g := sc.s.adjustCost(sc.k, 0, id["Grizzly Bears"], NoObj, sc.k.t.Cards[id["Grizzly Bears"]].Cost).Generic; g != 1 {
		t.Fatalf("Grizzly Bears at the offer: generic %d, want 1 (ValidSpell$ covers instants and sorceries only)", g)
	}
	sc.kcastBuild(bear, id["Grizzly Bears"], 0)
	if g := sc.s.adjustCost(sc.k, 0, id["Inspiration"], NoObj, full).Generic; g != 0 {
		t.Fatalf("after a creature cast: first Inspiration generic %d, want 0 (a creature does not spend the discount)", g)
	}
	sc.kcastBuild(insp, id["Inspiration"], 0)
	if g := sc.s.adjustCost(sc.k, 0, id["Inspiration"], NoObj, full).Generic; g != 3 {
		t.Fatalf("after an instant cast: next Inspiration generic %d, want 3 (the first-spell discount is spent)", g)
	}
}
