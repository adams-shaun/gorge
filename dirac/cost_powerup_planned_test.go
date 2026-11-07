package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPowerUpPlannedCostComposesStatics: gorge's planner prices a Power-up
// ability's printed cost without the entry-turn reduction (CR 702.193b) but
// with the cost statics (offerCostFor, CR 601.2f, 602.2b). Hulk, Gamma
// Goliath's {3} off She-Hulk, Jade Defender's {4}{G}{G} leaves {1}{G}{G},
// which three Forests and two other lands pay: the ability is offered.
func TestPowerUpPlannedCostComposesStatics(t *testing.T) {
	deck := []cardset.Entry{{Name: "Forest", Count: 20}}
	for _, n := range []string{"She-Hulk, Jade Defender", "Hulk, Gamma Goliath", "Mountain", "Castle Doom"} {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := cardsettest.Synth(t, "std-msh").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	sh := sc.perm(id["She-Hulk, Jade Defender"], 0)
	hulk := sc.perm(id["Hulk, Gamma Goliath"], 0)
	for _, n := range []string{"Forest", "Forest", "Forest", "Mountain", "Castle Doom"} {
		sc.perm(id[n], 0)
	}
	sc.nextTurn() // neither creature entered this turn: no CR 702.193b reduction
	if !sc.onBF(sh) || !sc.onBF(hulk) {
		t.Fatal("She-Hulk and Hulk are not both on the battlefield")
	}
	a := powerupAbility(t, sc, id["She-Hulk, Jade Defender"])
	if a.Cost.Flags&kop.CostPowerUp == 0 || a.Cost.Mana.MV() != 6 {
		t.Fatalf("She-Hulk's ability: flags %#x, mana value %d; want a Power-up costing 6", a.Cost.Flags, a.Cost.Mana.MV())
	}
	if got := sc.s.powerUpManaObj(sc.k, sh, a); got != a.Cost.Mana {
		t.Fatalf("She-Hulk did not enter this turn, yet her Power-up is reduced to %v", got)
	}
	if got := sc.s.abReduced(sc.k, a, sc.slot(sh), 0, a.Cost.Mana); got.MV() != 3 {
		t.Fatalf("Hulk's static prices She-Hulk's Power-up at mana value %d, want 3", got.MV())
	}
	d := sc.s.Step()
	if d.Kind != DkPriority || !powerupActivation(&d, sh) {
		t.Fatalf("five lands (three Forests) under Hulk: She-Hulk's {1}{G}{G} Power-up not offered (kind %d): %v", d.Kind, d.Options[:d.N])
	}
}
