package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// CR 701.67a: an announcement that taps too few helpers leaves a waterbend cost
// the mana window can't finish. Yue's Waterbend {5} with three Islands needs both
// Drones; with five Islands it needs neither.
func TestWaterbendAskRequiresTheTapsThePaymentNeeds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		islands, min int
	}{{3, 2}, {4, 1}, {5, 0}} {
		e, _, _ := newFixtureDeck(t, 9700, ppFiller)
		toMain1(t, e)
		src := onBoardCard(t, e, 0, corpusCard(t, "Yue, the Moon Spirit"))
		e.G.Obj(src).SummonSick = false
		for range 2 {
			e.G.Obj(onBoard(t, e, 0, apaDrone)).SummonSick = false
		}
		for range tc.islands {
			e.G.Obj(onBoard(t, e, 0, apaIsland)).SummonSick = false
		}
		d := ppPriority(t, e)
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "ability" && o.Obj == src {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("%d Islands: the ability is not offered: %+v", tc.islands, d.Options)
		}
		submitChoices(t, e, idx)
		ask := e.Pending()
		if ask == nil || ask.Kind != decision.KChoose || ask.Source != src {
			t.Fatalf("%d Islands: pending %+v, want the helper ask", tc.islands, ask)
		}
		if ask.Min != tc.min || ask.Max != 2 {
			t.Errorf("%d Islands: the helper ask takes %d to %d, want %d to 2", tc.islands, ask.Min, ask.Max, tc.min)
		}
	}
}
