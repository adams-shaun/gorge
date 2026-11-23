package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// deferredWatcherScen is synthScen over pool tail-70 with Price of Glory's
// TrigTaps watcher forced to kop.TapPlanDefer. Price of Glory's destroy-self
// body is a vetted row that kgen lowers TapPlanOrdinary (the tapped land stays
// an ordinary payment unit; CR 405.6c/603.2, 605.4a), and no other card in the
// pool watches taps, so the deferred-source census-gap scenarios stamp the
// class a deferring TapsForMana watcher carries (Activator$ Player.NonActive
// filter and all) onto this card's line before the state is built.
func deferredWatcherScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	set := synthSet(t, "tail-70")
	var deck []cardset.Entry
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	pog := id["Price of Glory"]
	cd := &tbl.Cards[pog]
	cd.Abilities = slices.Clone(cd.Abilities) // the table's slices may be shared
	patched := false
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps {
			if a.Trig.TapPlan != kop.TapPlanOrdinary {
				t.Fatalf("precondition: Price of Glory's watcher is TapPlan %d, not the ordinary destroy-self row", a.Trig.TapPlan)
			}
			a.Trig.TapPlan = kop.TapPlanDefer
			patched = true
		}
	}
	if !patched {
		t.Fatal("precondition: Price of Glory has no TrigTaps watcher")
	}
	return newScenTable(t, tbl, Config{NoElide: true}), id
}
