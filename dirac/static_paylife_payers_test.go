package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPayLifeGrantNonRiderPayers: the planner's discount for K'rrik's
// PayLifeInsteadOf:B grant (tapNeed) and the life that settles it are one
// unit for every payer that taps through the planner, not only payRider's
// plain arm. {1}{B} with two Plains and no black source: the plan taps one
// Plains for the generic, and the {B} pip it left off the taps is 2 life.
// payRiderRestr (a table with a restricted source) and the special-action
// payers once paid the {B} pip from a pool with no {B} and panicked.
func TestPayLifeGrantNonRiderPayers(t *testing.T) {
	set := cardsettest.Set(t, "staticzones")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, c := range []string{"K'rrik, Son of Yawgmoth", "Plains"} {
		if _, ok := id[c]; !ok {
			t.Fatalf("%s not in the staticzones table", c)
		}
	}
	payers := []struct {
		name string
		pay  func(sc *scen, cost *kop.ManaCost)
	}{
		{"payRiderRestr", func(sc *scen, cost *kop.ManaCost) { sc.s.payRiderRestr(sc.k, 0, cost, 0, NoObj, false) }},
		{"payRider", func(sc *scen, cost *kop.ManaCost) { sc.s.payRider(sc.k, 0, cost, 0, NoObj, false) }},
		{"paySpecial", func(sc *scen, cost *kop.ManaCost) {
			sc.s.tapFor(sc.k, 0, cost, 0, NoObj)
			sc.s.paySpecial(sc.k, 0, cost, 0, NoSlot, 0)
		}},
	}
	for _, py := range payers {
		t.Run(py.name, func(t *testing.T) {
			sc := newScenTable(t, tbl, Config{NoElide: true})
			sc.perm(id["K'rrik, Son of Yawgmoth"], 0)
			sc.perm(id["Plains"], 0)
			sc.perm(id["Plains"], 0)
			var cost kop.ManaCost
			cost.Generic, cost.Pips[kop.B] = 1, 1
			if !sc.s.payLifeGrantApplies(sc.k, 0, &cost) {
				t.Fatal("precondition: the grant does not apply to the payer")
			}
			before := sc.s.pl[0].life
			py.pay(sc, &cost)
			if got := before - sc.s.pl[0].life; got != 2 {
				t.Errorf("life paid %d, want 2 (the {B} pip's life)", got)
			}
			if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
				t.Errorf("pool %v left floating, want it spent", pool)
			}
		})
	}
}
