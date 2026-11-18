package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWeatherMakerUnlessCtrFee: Weather Maker's "{T}, Remove two charge
// counters: Add {C}{C}" is a fixed, source-anchored SubCounter fee on a fixed
// production, which gorge's unless census prices (rules paid_mana_census.go
// unlessWindowUnits' SubCounter arm, pay.ManaSubCounterOnlyCost), so the
// kernel's unless / ward probe must answer the same offers
// (windowAffordsTake): the gate takes the walk for the counter-cost source
// (unlessCtrGaps) and the probe's windowPick answers it with the counter
// activation (ctrCensusOffer, ctrGap) -- not with the plain {T} first
// ability, whose one mana leaves the fee unpriced and the offer decline only
// where gorge offers pay. The cast-payment probe (windowAffords) keeps the
// fee out: the arm is the unless census's, the shared windowManaUnits
// withholds the shape as it withholds the choice-shaped plain one beside it.
func TestWeatherMakerUnlessCtrFee(t *testing.T) {
	set := cardsettest.Set(t, "ctrfilter")
	ent := func(names ...string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(ent("Weather Maker"), ent("Plains"))
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == name {
				return CardID(i)
			}
		}
		t.Fatalf("%s not in the table", name)
		return 0
	}
	build := func() (*scen, Slot) {
		sc := newScenTableLib(t, tbl, id("Plains"), Config{})
		wm := sc.perm(id("Weather Maker"), 0)
		sl := sc.slot(wm)
		pm := &sc.s.perms[sl]
		pm.seeCtr(sc.s)
		return sc, sl
	}
	setCtr := func(sc *scen, sl Slot, n int) {
		sc.s.placeCounters(sc.k, sl, int(kop.CtrCharge), n, 0, ctrPutRaw)
	}

	// Two charge counters, no other source: the fee alt alone pays an unless
	// {2}, so gorge offers the pay; the fee alt is two and the plain tap one,
	// so an unless {3} stays decline (two alts of one source are one tap) and
	// the cast-payment probe keeps the shape out (the arm is the unless
	// census's, not the shared one's).
	sc, sl := build()
	setCtr(sc, sl, 2)
	sc = &scen{s: sc.s, k: sc.s.k()}
	waN := func(amt int, pips int16, unless bool) bool {
		ok, _ := sc.s.windowAffordsN(sc.k, 0, amt, pips, unless)
		return ok
	}
	wa := func(c *kop.ManaCost) bool {
		ok, _ := sc.s.windowAffords(sc.k, 0, c)
		return ok
	}
	if !waN(2, 0, true) {
		t.Fatal("unless {2} with Weather Maker holding two charge counters: want the fixed counter-fee source priced")
	}
	if waN(3, 0, true) {
		t.Fatal("unless {3} with no other source: want decline")
	}
	if wa(&[]kop.ManaCost{manaOf(2, 0)}[0]) {
		t.Fatal("cast probe {2}: want decline (the fee shape is the unless census's, not the shared one's)")
	}

	// One charge counter is too few to remove: the fee alt stays out and the
	// plain tap alone cannot pay {2}.
	sc, sl = build()
	setCtr(sc, sl, 1)
	sc = &scen{s: sc.s, k: sc.s.k()}
	if waN(2, 0, true) {
		t.Fatal("unless {2} with one charge counter: want decline (the fee needs two, the plain tap one)")
	}

	// A ward {3}: the fee alt plus one more untapped source pays it (CR
	// 702.21a's window is the same census), which the fee-less probe missed.
	sc, sl = build()
	setCtr(sc, sl, 2)
	sc.perm(id("Plains"), 0)
	sc = &scen{s: sc.s, k: sc.s.k()}
	if !waN(3, 0, false) {
		t.Fatal("ward {3} with Weather Maker and one land: want the fee source priced")
	}
	if !waN(2, 0, true) {
		t.Fatal("unless {2} beside the land: want payable")
	}

	// Tapped, the source is spent: neither alternative is offered and the
	// lone land's mana cannot pay {2}.
	sc.s.perms[sl].flags |= pfTapped
	sc = &scen{s: sc.s, k: sc.s.k()}
	if waN(2, 0, true) {
		t.Fatal("unless {2} with Weather Maker tapped: want decline")
	}
}
