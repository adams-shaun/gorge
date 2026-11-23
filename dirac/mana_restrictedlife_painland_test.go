package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// restrLifePainScen: Untaidake, the Cloud Keeper ("{T}, Pay 2 life: Add {C}{C}",
// spend only on legendary spells), a Scabland, two Islands and a Plains, with
// Spider-Man, Peter Parker ({3}{W}{W}, legendary) in hand at the given life.
// The printed sources make four mana and the second white pip is the Scabland's
// {W}, which deals 1 damage: the only plan is the Keeper's 2 life plus that 1.
func restrLifePainScen(t *testing.T, life int16) (sc *scen, spider Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Untaidake, the Cloud Keeper", Count: 1}, {Name: "Spider-Man, Peter Parker", Count: 1},
		{Name: "Scabland", Count: 1}, {Name: "Island", Count: 2}, {Name: "Plains", Count: 10}}
	tbl, _, _, err := cardsettest.Set(t, "restrictedlifepain").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	cfg := LockstepConfig(SurfaceAutopay)
	cfg.NoElide = true
	sc = newScenTableLib(t, tbl, id["Plains"], cfg)
	for _, n := range []string{"Untaidake, the Cloud Keeper", "Scabland", "Island", "Island", "Plains"} {
		sc.perm(id[n], 0)
	}
	for sl := range sc.s.perms {
		sc.s.perms[sl].flags &^= pfTapped // both enter-tapped lands are ready
	}
	spider = sc.hand(id["Spider-Man, Peter Parker"], 0)
	sc.s.pl[0].life = life
	return sc, spider
}

// CR 119.4, 120.3: gorge's planner sums the restricted unit's life with the
// damage of the painful source the same plan taps, and cuts a plan that
// reaches the payer's life. At 3 life the only plan pays 2 + 1 and is lethal,
// so the cast is not offered; at 4 life it is (tail-73-c000 pair 92, decision
// 358 of the autopay game).
func TestRestrictedLifeUnitPainlandLethalNotOffered(t *testing.T) {
	ctl, spiderCtl := restrLifePainScen(t, 4)
	if !offersCast(ctl.s, spiderCtl) {
		t.Fatal("precondition: the 4-life board does not offer the cast its 3-life plan pays")
	}
	sc, spider := restrLifePainScen(t, 3)
	if offersCast(sc.s, spider) {
		t.Fatal("the cast is offered at 3 life where the only plan pays 2 life and 1 damage (lethal)")
	}
}
