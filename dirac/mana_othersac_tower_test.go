package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// otherSacTowerScen is seat 0's main phase with a Phyrexian Tower, a Syr
// Konrad, the Grim ({1}{B}: each player mills a card) and a Plains on its
// side, plus the named extra permanents; it returns the scenario, Konrad and
// the index of his {1}{B} activation.
func otherSacTowerScen(t *testing.T, extra ...string) (*scen, Obj, uint8) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Phyrexian Tower", Count: 1}, {Name: "Syr Konrad, the Grim", Count: 1},
		{Name: "Carrion Feeder", Count: 1}, {Name: "Swamp", Count: 1}, {Name: "Plains", Count: 20},
	}
	tbl, da, _, err := cardsettest.Set(t, "hosted-cmdr").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, n := range []string{"Phyrexian Tower", "Syr Konrad, the Grim", "Carrion Feeder", "Swamp", "Plains"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: %s is not in the matchup table", n)
		}
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true, GorgeAutopaySurface: true})
	sc.perm(id["Phyrexian Tower"], 0)
	sc.perm(id["Plains"], 0)
	konrad := sc.perm(id["Syr Konrad, the Grim"], 0)
	for _, n := range extra {
		sc.perm(id[n], 0)
	}
	ab := -1
	for i, a := range tbl.Cards[id["Syr Konrad, the Grim"]].Abilities {
		if a.Kind == kop.AbActivated && a.Cost.Mana.MV() == 2 {
			ab = i
		}
	}
	if ab < 0 {
		t.Fatal("precondition: Syr Konrad, the Grim has no {1}{B} activation")
	}
	return sc, konrad, uint8(ab)
}

// TestOtherSacPlanDefersTowerProduction: gorge's payment planner defers a
// fixed colour behind the {T} (pay.planOtherSacProductionOK, Phyrexian
// Tower's "{T}, Sacrifice a creature: Add {B}{B}"), so Konrad's {1}{B}, which
// only the Tower's sacrifice of Konrad himself could fund beside a Plains,
// has no plan and is not offered: the kernel's plan-source predicate
// (planOtherSacSource, kop.OtherSacPlanProduction) defers the same
// production, so the planned other-sacrifice arm (actOtherSacAfford) prices no Tower unit. The positive
// control pays the {1}{B} with a Swamp, so the probe can see an offer.
//
// Beside a Carrion Feeder the activation IS offered, in both engines: gorge's
// planner still declines it (source:last_resort), but its census misses the
// Tower, so PotentialPaymentPlans prices it with the prefix script "activate
// the Tower, its Sacrifice a creature: Add {B}{B} ability, sacrifice the
// Feeder" (potentialCensusFallback -> potentialPrefixScript), and the oracle
// adds that scripted play to gorge's offers (dirac/oracle/gorge.go, prScript).
// The kernel's tap-another script walk prices the same line. (The posed
// priority decision alone does not list it: askPriority walks with no pending
// decision, so autoPayOffer's census arm runs no search; the oracle's merge is
// the surface the kernel mirrors, as in the script_prefix_sac_breaks_present_gate
// repro, Sliver Hive funded by the Tower's sacrifice of another creature.)
func TestOtherSacPlanDefersTowerProduction(t *testing.T) {
	t.Run("Konrad alone", func(t *testing.T) {
		sc, konrad, ab := otherSacTowerScen(t)
		if sc.activateOffered(konrad, ab) {
			t.Fatal("the activation is offered: the Tower's {B}{B} is planned where gorge defers it")
		}
	})
	t.Run("beside a Carrion Feeder", func(t *testing.T) {
		sc, konrad, ab := otherSacTowerScen(t, "Carrion Feeder")
		if !sc.activateOffered(konrad, ab) {
			t.Fatal("the activation is withheld though gorge's prefix script (the Tower's {B}{B}, sacrificing the Feeder) prices it")
		}
	})
	t.Run("a Swamp pays it", func(t *testing.T) {
		sc, konrad, ab := otherSacTowerScen(t, "Swamp")
		if !sc.activateOffered(konrad, ab) {
			t.Fatal("precondition: the activation is withheld though the Plains and a Swamp pay its {1}{B}")
		}
	})
}
