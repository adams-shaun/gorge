package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// loreseekers_stone_raisecost_test.go pins Loreseeker's Stone's own-line
// RaiseCost$ ({3}, {T}: Draw three cards. This ability costs {1} more to
// activate for each card in your hand -- CR 601.2f via 602.2b): the
// activation cost is {3}+{1} per card in hand, so with two other cards in
// hand it is {5} and four mana sources cannot pay it, while five can and the
// payment taps all five. The only corpus carrier of `RaiseCost$` on an
// A:AB line.

func TestLoreseekersStoneRaiseCostPerCardInHand(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _, ids := realCardEngine(t, reg, 218, "Loreseeker's Stone")
	stone := ids[0]

	// Empty seat 0's hand (the opening draw went to the library), then load
	// exactly two other cards: the raise is {1} per card in hand.
	for _, id := range e.G.Zone(state.ZHand, 0) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	hand := func() int { return len(e.G.Zone(state.ZHand, 0)) }
	if hand() != 0 {
		t.Fatalf("precondition: hand = %d cards, want 0", hand())
	}
	c, _ := reg.Lookup("Goblin Guide")
	putCardIn(t, e, 0, c, state.ZHand, false)
	putCardIn(t, e, 0, c, state.ZHand, false)
	if hand() != 2 {
		t.Fatalf("precondition: hand = %d cards, want 2", hand())
	}

	// Four mana sources, empty pool, untapped Stone: {3}+{2} = {5} > 4.
	// The activation must NOT be offered.
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: pool = %d, want 0", e.G.Players[0].Pool.Total())
	}
	c, _ = reg.Lookup("Plains")
	var lands []state.ObjID
	for i := 0; i < 4; i++ {
		lands = append(lands, putCardIn(t, e, 0, c, state.ZBattlefield, false))
	}
	e.priorityRound()
	if o := loreseekerAbilityOffer(e, stone); o != nil {
		t.Fatalf("Loreseeker's Stone with 4 lands and 2 cards in hand costs {5} > 4 but was offered: %+v", e.Pending().Options)
	}

	// A fifth mana source makes it exactly payable: {5} for {3}+{2}.
	lands = append(lands, putCardIn(t, e, 0, c, state.ZBattlefield, false))
	e.priorityRound()
	o := loreseekerAbilityOffer(e, stone)
	if o == nil {
		t.Fatalf("Loreseeker's Stone with 5 lands must be offered: %+v", e.Pending().Options)
	}
	// The displayed cost is the raised one: {3}+{2} and the Stone's tap.
	if o.Cost != "5 T" {
		t.Fatalf("offered cost = %q, want the raised %q ({3}+{1} per card in hand)", o.Cost, "5 T")
	}
	submitChoices(t, e, o.Index)
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("after activating Loreseeker's Stone: %+v", d)
	}
	for _, id := range lands {
		if !e.G.Obj(id).Tapped {
			t.Fatalf("land %d untapped after paying {3}+{2}; want all five tapped", id)
		}
	}
	if e.G.Obj(stone).Zone != state.ZBattlefield {
		t.Fatalf("stone zone = %s", e.G.Obj(stone).Zone)
	}
}

// putCardIn mints its objects outside the event log (game.go AddObject), so
// replayCheck cannot compare this game; no log-only replay is asserted here.

// loreseekerAbilityOffer returns the ability option for id off the pending
// priority decision, or nil when none is offered.
func loreseekerAbilityOffer(e *Engine, id state.ObjID) *decision.Option {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return nil
	}
	for i := range d.Options {
		if o := &d.Options[i]; o.Kind == "ability" && o.Obj == id {
			return o
		}
	}
	return nil
}
