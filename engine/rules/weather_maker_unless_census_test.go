package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Weather Maker unless-pay census regression: "{T}, Remove two charge
// counters from this artifact: Add {C}{C}" is a fixed, source-anchored
// SubCounter fee on a fixed production (the shape
// pay.ManaSubCounterOnlyCost prices), so the unless census lists it beside
// the plain "{T}: Add one mana of any color" alternative and the offer gate
// offers the pay of an unless {2} on the fee alternative alone -- with the
// pool empty and no other source untapped. The fee's counter is the source's
// own: one charge counter is too few to remove, and the shape stays out (fail
// closed), the plain tap alone not paying {2}. The kernel's mirror is
// dirac/mana_countercost.go ctrCensusAb (TestWeatherMakerUnlessCtrFee).
func TestWeatherMakerUnlessCensusPricesCtrFee(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Weather Maker"), lookup(t, reg, "Plains")}, nil)
	wm := moveByName(t, e, 0, "Weather Maker", state.ZBattlefield)
	e.G.Obj(wm).AddCounter("CHARGE", 2)

	if !e.UnlessCostPayable(0, "{2}") {
		t.Fatal("unless {2} with Weather Maker holding two charge counters: want the pay offered")
	}
	if e.UnlessCostPayable(0, "{3}") {
		t.Fatal("unless {3} with no other source: want decline (two alternatives of one source are one tap)")
	}
	var fee *windowManaAlt
	for _, u := range e.unlessWindowUnits(0) {
		if u.ID != wm {
			continue
		}
		for i := range u.Alts {
			a := &u.Alts[i]
			if a.Any || a.Amt != 2 || a.Counts[5] != 1 {
				continue
			}
			fee = a
		}
	}
	if fee == nil {
		t.Fatal("unless census: want Weather Maker's {C}{C} fee alternative listed")
	}

	e2 := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Weather Maker")}, nil)
	wm2 := moveByName(t, e2, 0, "Weather Maker", state.ZBattlefield)
	e2.G.Obj(wm2).AddCounter("CHARGE", 1)
	if e2.UnlessCostPayable(0, "{2}") {
		t.Fatal("unless {2} with one charge counter: want decline (the fee needs two, the plain tap one)")
	}
}
