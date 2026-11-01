package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCycledTrigger: a cycling ability's discard-this cost (AfCycling)
// fires the card's own Tf2Cycled trigger from the graveyard, and a
// battlefield watcher's "whenever you cycle" beside its "whenever you
// discard" (one walk, ability order); a non-cycling discard-this cost (a
// channel-style hand ability) fires only the discard trigger.
func TestCycledTrigger(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(2)}}
	c, w := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Cycler", Types: kop.TInstant, Abilities: []kop.Ability{
		{Kind: kop.AbActivated, AFlags: kop.AfCycling, ActZone: kop.ZoneHand, Cost: kop.Cost{Flags: kop.CostDiscardSelf}},
		{Kind: kop.AbActivated, ActZone: kop.ZoneHand, Cost: kop.Cost{Flags: kop.CostDiscardSelf}},
		{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDiscarded, Flags: kop.TfSelf, Flags2: kop.Tf2Cycled, Zone: kop.ZoneGraveyard}, Prog: gain}}},
		kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDiscarded, Flags2: kop.Tf2Cycled}, Prog: gain},
			{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDiscarded}, Prog: gain}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(w, 0)
	for ab, want := range []int{3, 1} {
		o := sc.obj(c, 0)
		sc.s.detach(o)
		sc.s.place(o, kop.ZoneHand, false)
		sc.s.npend = 0
		e := stackEntry{kind: seAbility, obj: o, card: c, ab: uint8(ab), ctrl: 0, trigObj: NoObj}
		sc.s.discardSelfCost(sc.k, &e)
		if sc.s.objs[o].zone != kop.ZoneGraveyard || int(sc.s.npend) != want {
			t.Fatalf("ability %d: zone %d, %d pending, want graveyard and %d", ab, sc.s.objs[o].zone, sc.s.npend, want)
		}
	}
}
