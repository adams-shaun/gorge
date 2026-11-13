package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A counter-cost mana gap (Weather Maker) beside a first-only filter
// (Omni-Cheese Pizza) funds its fee and the rest of Leonardo, Leader in
// Blue's {1}{W} activation (mana_mixgaps.go mixCtr, otherCtr; CR 605.3a,
// 601.2g-h). gorge's one prefix search activates the census gaps in
// battlefield order: Weather Maker's "{T}, remove two charge counters: Add
// {C}{C}", one {C} paying Pizza's {1}, the other the activation's generic.
func ctrFilterBoard(t *testing.T, counters int) (*scen, Obj, Obj, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "ctrfilter")
	deck := []cardset.Entry{
		{Name: "Plains", Count: 1},
		{Name: "Weather Maker", Count: 1},
		{Name: "Omni-Cheese Pizza", Count: 1},
		{Name: "Leonardo, Leader in Blue", Count: 1},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// No land on the battlefield: the two artifacts are the only sources.
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	wm := sc.perm(id["Weather Maker"], 0)
	if !sc.s.perms[sc.slot(wm)].setCtrN(sc.s, int(kop.CtrCharge), counters) {
		t.Fatalf("cannot put %d charge counters on Weather Maker", counters)
	}
	pizza := sc.perm(id["Omni-Cheese Pizza"], 0)
	leo := sc.perm(id["Leonardo, Leader in Blue"], 0)
	return sc, wm, pizza, leo
}

// ctrFilterOffered steps to p0's priority decision and returns Leonardo's
// activation when the play is on it.
func ctrFilterOffered(t *testing.T, sc *scen, leo Obj) (Decision, Action, bool) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == leo {
			return d, a, true
		}
	}
	return d, 0, false
}

// TestCounterGapFundsFilterFee: Weather Maker's two charge counters pay
// Omni-Cheese Pizza's {1} for its {W} and Leonardo's remaining {1}, so the
// {1}{W} activation is offered; played, Pizza is sacrificed, Weather Maker is
// tapped with no counters, and the pool is empty. With one counter Weather
// Maker's counter ability is not activatable, so the two artifacts cannot pay
// the activation and it is not offered.
func TestCounterGapFundsFilterFee(t *testing.T) {
	sc, wm, pizza, leo := ctrFilterBoard(t, 2)
	if !sc.s.scriptCtr(sc.k, 0).has(sc.slot(wm)) {
		t.Fatal("Weather Maker is not a live counter-cost gap with two charge counters")
	}
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(pizza)) {
		t.Fatal("Omni-Cheese Pizza is not a census filter gap")
	}
	d, act, ok := ctrFilterOffered(t, sc, leo)
	if !ok {
		t.Fatalf("Leonardo's {1}{W} activation not offered: %v", d.Options[:d.N])
	}
	sc.s.Apply(act)
	// beginActivate builds the payment (the cost is paid at the next stage)
	sc.s.Step()
	if z := sc.s.objs[pizza].zone; z != kop.ZoneGraveyard {
		t.Errorf("Pizza is in zone %d after the activation, want the graveyard", z)
	}
	if sc.s.perms[sc.slot(wm)].flags&pfTapped == 0 {
		t.Error("Weather Maker is untapped after the activation")
	}
	if n := sc.s.perms[sc.slot(wm)].ctrN(sc.s, int(kop.CtrCharge)); n != 0 {
		t.Errorf("Weather Maker holds %d charge counters, want 0", n)
	}
	if got := poolTotal(&sc.s.pl[0]); got != 0 {
		t.Errorf("the pool holds %d mana after the activation, want none: %v", got, sc.s.pl[0].pool)
	}

	sc, wm, pizza, leo = ctrFilterBoard(t, 1)
	if sc.s.scriptCtr(sc.k, 0).has(sc.slot(wm)) {
		t.Fatal("Weather Maker is a live counter-cost gap with only one charge counter")
	}
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(pizza)) {
		t.Fatal("Omni-Cheese Pizza is not a census filter gap with one counter")
	}
	if d, _, ok := ctrFilterOffered(t, sc, leo); ok {
		t.Fatalf("Leonardo's {1}{W} activation offered with only one charge counter: %v", d.Options[:d.N])
	}
}
