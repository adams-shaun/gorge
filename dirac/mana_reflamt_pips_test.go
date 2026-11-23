package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReflAmtUnitFundsColouredPipsAtOneLife (CR 106.1, 119.4): Incubation Druid
// with a +1/+1 counter adds three mana of one type, so beside Ifnir Deadlands,
// Nomad Stadium and two Swamps -- the controller at 1 life, where Ifnir's {B}
// (1 life) and Nomad's {W} (1 damage) are withheld -- Vengeant Vampire
// ({4}{B}{B}) is payable by the Druid's {B}{B}{B}, the Swamps' {B}{B} and
// Ifnir's {C}. The Druid is a planner unit of its reflected colours at the
// evaluated amount (mana_reflamt.go), one pick for all three mana; without the
// counter it makes one mana and the cast is out of reach.
func TestReflAmtUnitFundsColouredPipsAtOneLife(t *testing.T) {
	offered := func(counters int) bool {
		deck := []cardset.Entry{
			{Name: "Vengeant Vampire", Count: 1}, {Name: "Incubation Druid", Count: 1}, {Name: "Ifnir Deadlands", Count: 1},
			{Name: "Nomad Stadium", Count: 1}, {Name: "Swamp", Count: 2}, {Name: "Forest", Count: 1},
		}
		tbl, da, _, err := cardsettest.Set(t, "reflamtpips").Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		cfg := LockstepConfig(SurfaceFull)
		cfg.NoElide = true
		sc := newScenTableLib(t, tbl, id["Forest"], cfg)
		druid := sc.perm(id["Incubation Druid"], 0)
		for _, n := range []string{"Ifnir Deadlands", "Nomad Stadium", "Swamp", "Swamp"} {
			sc.perm(id[n], 0)
		}
		if counters > 0 {
			sc.s.perms[sc.slot(druid)].addCtrN(sc.s, int(kop.CtrP1P1), counters)
		}
		sc.s.pl[0].life = 1
		if pc := sc.s.manaPCGen(sc.k, sc.slot(druid)); pc.manaAb < 0 || !pc.manaDyn || int(pc.manaN) != max(1, 3*counters) {
			t.Fatalf("precondition: the Druid's view %+v is not its evaluated-amount unit (counters %d)", pc, counters)
		}
		return sc.castOffered(0, sc.hand(id["Vengeant Vampire"], 0))
	}
	if !offered(1) {
		t.Error("Vengeant Vampire is not offered: the Druid's {B}{B}{B} funds the coloured pips")
	}
	if offered(0) {
		t.Error("Vengeant Vampire is offered with a counterless Druid (one mana)")
	}
}
