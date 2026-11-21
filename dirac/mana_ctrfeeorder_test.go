package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCtrGapBesideFeeFilterFeeOrder: Vivid Grove and Vivid Marsh (counter-cost
// any-colour gaps) beside Izzet Signet ({1},{T}: {U}{R}, a fee gap) and four
// Islands. gorge's plain prefix run spends its 160 nodes on the fee's branches;
// its fee-order run then finds {B}{B} (Marsh's {B}, Grove's counter) in 15. The
// kernel's feeOrderFallback used to decline with any other gap on the board and
// offered neither cast.
func TestCtrGapBesideFeeFilterFeeOrder(t *testing.T) {
	set := cardsettest.Synth(t, "tail-47")
	deck := []cardset.Entry{{Name: "Vivid Grove", Count: 1}, {Name: "Vivid Marsh", Count: 1}, {Name: "A-The Meathook Massacre", Count: 1},
		{Name: "Phyrexian Defiler", Count: 1}, {Name: "Izzet Signet", Count: 1}, {Name: "Island", Count: 20}, {Name: "Swamp", Count: 4}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	grove := sc.perm(id["Vivid Grove"], 0)
	marsh := sc.perm(id["Vivid Marsh"], 0)
	for _, o := range []Obj{grove, marsh} {
		sc.s.perms[sc.slot(o)].flags &^= pfTapped // they enter tapped
	}
	for i := 0; i < 4; i++ {
		sc.perm(id["Island"], 0)
	}
	sc.perm(id["Izzet Signet"], 0)
	mass := sc.hand(id["A-The Meathook Massacre"], 0)
	def := sc.hand(id["Phyrexian Defiler"], 0)

	// Preconditions: both Vivids hold their counters and are live counter gaps,
	// so the assertion is about the mixed board's fee-order run.
	for _, o := range []Obj{grove, marsh} {
		if n := sc.s.perms[sc.slot(o)].ctrN(sc.s, int(kop.CtrCharge)); n != 2 {
			t.Fatalf("precondition: a Vivid land holds %d charge counters, want 2", n)
		}
		if !sc.s.scriptCtr(sc.k, 0).has(sc.slot(o)) {
			t.Fatal("precondition: a Vivid land is not a live counter-cost gap")
		}
	}
	d := sc.s.Step()
	var gotMass, gotDef bool
	for _, a := range d.Options[:d.N] {
		if a.Kind() != AkCast {
			continue
		}
		gotMass = gotMass || a.Source() == TCode(mass)
		gotDef = gotDef || a.Source() == TCode(def)
	}
	if !gotMass || !gotDef {
		t.Errorf("want both casts offered (Massacre %v, Defiler %v): %v", gotMass, gotDef, d.Options[:d.N])
	}
}
