package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
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

// An activation whose cost has {T} can't also tap its source for mana (CR 602.2b):
// the ask requires the taps that leaves short, and the source taps once.
func TestOwnTapSourcePaysNoMana(t *testing.T) {
	t.Parallel()
	const bender = "Name:Test Bender\nTypes:Creature Elf\nPT:1/1\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | Amount$ 2 | SpellDescription$ Two.\n" +
		"A:AB$ GainLife | Cost$ Waterbend<3> T | LifeAmount$ 1 | SpellDescription$ Life.\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 9700, ppFiller)
	toMain1(t, e)
	src := onBoard(t, e, 0, bender)
	e.G.Obj(src).SummonSick = false
	for range 2 {
		e.G.Obj(onBoard(t, e, 0, apaDrone)).SummonSick = false
	}
	e.G.Obj(onBoard(t, e, 0, apaIsland)).SummonSick = false
	d := ppPriority(t, e)
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == src {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the ability is not offered: %+v", d.Options)
	}
	start := len(e.L.Events)
	submitChoices(t, e, idx)
	ask := e.Pending()
	if ask == nil || ask.Kind != decision.KChoose || ask.Min != 2 {
		t.Fatalf("pending %+v, want the helper ask requiring both Drones", ask)
	}
	submitChoices(t, e, 0, 1)
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.PaymentFallback != nil || len(e.G.Stack) != 1 {
		t.Fatalf("pending %+v, stack %v: want the ability on the stack, paid", d, e.G.Stack)
	}
	taps := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.Tap && ev.Obj == src {
			taps++
		}
	}
	if taps != 1 {
		t.Errorf("the source tapped %d times, want once", taps)
	}
}
