package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// otherSacTargetScen is seat 0's main phase with Fateful Absence ({1}{W}, one
// target creature or planeswalker) in hand, an Ashnod's Altar
// on its side, and the creatures in fodder on the
// battlefield; it returns the scenario, the spell and the creatures.
func otherSacTargetScen(t *testing.T, fodder ...string) (*scen, Obj, []Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Fateful Absence", Count: 1}, {Name: "Ashnod's Altar", Count: 1},
		{Name: "Birds of Paradise", Count: 1}, {Name: "Carrion Feeder", Count: 1}, {Name: "Plains", Count: 20},
	}
	tbl, da, _, err := cardsettest.Set(t, "hosted-cmdr").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, GorgeAutopaySurface: true})
	sc.perm(id["Ashnod's Altar"], 0)
	var cs []Obj
	for _, n := range fodder {
		cs = append(cs, sc.perm(id[n], 0))
	}
	return sc, sc.hand(id["Fateful Absence"], 0), cs
}

// TestOtherSacPlanCastOfferPricesTheTargetsProduction: gorge plans an
// Ashnod's Altar step with victims that are not plan sources
// (pay.AssignPlanVictims), before any target is asked. A cast whose {1}{W} only
// the altar's {C}{C} beside a mana creature's {W} pays, with that creature the
// spell's only target and the altar's only fodder, has no plan: the creature is
// a plan source, so it is no victim, and the potential route's script (tap it,
// sacrifice it) consumes the target (CR 601.2c). The kernel's tier-1 census
// priced the altar anyway and offered the cast. A second creature is a victim:
// the plan exists, and the cast is offered.
func TestOtherSacPlanCastOfferPricesTheTargetsProduction(t *testing.T) {
	t.Run("the only fodder is the target and a plan source", func(t *testing.T) {
		sc, spell, _ := otherSacTargetScen(t, "Birds of Paradise")
		if sc.castOffered(0, spell) {
			t.Fatal("the cast is offered: its altar unit sacrifices the Birds the plan taps for {W}")
		}
	})
	t.Run("another creature is the victim", func(t *testing.T) {
		sc, spell, _ := otherSacTargetScen(t, "Birds of Paradise", "Carrion Feeder")
		if !sc.castOffered(0, spell) {
			t.Fatal("the cast is withheld: the Carrion Feeder is the altar's victim and the Birds pay {W}")
		}
	})
}
