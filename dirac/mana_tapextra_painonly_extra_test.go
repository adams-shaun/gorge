package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestTapExtraPainOnlyCensusPaysTheExtra: a pain-only {C}{C} source
// (Ancient Tomb) gets the TapExtra {C} (Forsaken Monument) on its one tap.
// The census must price it at three, not two, or a seven-mana cast off
// Ancient Tomb and four lands is not offered (CR 601.2g, 605.1b).
func TestTapExtraPainOnlyCensusPaysTheExtra(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Ancient Tomb", Count: 1},
		{Name: "Forsaken Monument", Count: 1},
		{Name: "Sire of Seven Deaths", Count: 1},
		{Name: "Swamp", Count: 4},
	}
	tbl, da, _, err := cardsettest.Set(t, "hosted-cmdr").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true, GorgeAutopaySurface: true})
	sc.perm(id["Ancient Tomb"], 0)
	sc.perm(id["Forsaken Monument"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	sire := sc.hand(id["Sire of Seven Deaths"], 0)
	if !sc.k.painMana || !sc.k.gen.anyTapExtra {
		t.Fatalf("precondition: pain mana %v, tap extra %v; want both", sc.k.painMana, sc.k.gen.anyTapExtra)
	}
	if !offersCast(sc.s, sire) {
		t.Fatal("the seven-mana cast is not offered: Ancient Tomb's extra {C} is not in the census")
	}
}
