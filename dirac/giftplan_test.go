package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func giftPlanScen(t *testing.T, cfg Config, lands int) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Starforged Sword", Count: 1}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "blb")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, cfg)
	for i := 0; i < lands; i++ {
		sc.perm(id["Island"], 0)
	}
	return sc, sc.hand(id["Starforged Sword"], 0)
}

func TestGiftSpellPlanned(t *testing.T) {
	sc, sword := giftPlanScen(t, Config{GorgeAutopaySurface: true, NoElide: true}, 4)
	if !sc.s.FeatPlannedCast(0, sword, 0) {
		t.Fatalf("FeatPlannedCast(Starforged Sword) = false; gorge's planner prices a Gift spell")
	}
	if !offersCast(sc.s, sword) {
		t.Fatalf("autopay surface does not offer Starforged Sword from lands")
	}
	sc, sword = giftPlanScen(t, Config{NoElide: true}, 4)
	if !offersCast(sc.s, sword) {
		t.Fatalf("kernel surface does not offer Starforged Sword")
	}
	sc, sword = giftPlanScen(t, Config{GorgeLowering: true, NoElide: true}, 4)
	if !offersCast(sc.s, sword) {
		t.Fatalf("lowering surface does not offer Starforged Sword")
	}
	sc, sword = giftPlanScen(t, Config{GorgeAutopaySurface: true, NoElide: true}, 3)
	if offersCast(sc.s, sword) {
		t.Fatalf("autopay surface offers Starforged Sword short of its cost")
	}
}
