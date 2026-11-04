package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDanielsSelfBottomCostAndGraveyardActivation pins Daniels, Temporal
// Agent's "{2}, Put this card from your graveyard on the bottom of your
// library: You gain 2 life" (CR 118.3, 602.2b): the ability is not offered
// from the battlefield, is offered from the graveyard, and paying puts the
// source at the BOTTOM of its owner's library without touching the order of
// the other cards (a shuffle is not a cost).
func TestDanielsSelfBottomCostAndGraveyardActivation(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	daniels := lookup(t, reg, "Daniels, Temporal Agent")
	bears := lookup(t, reg, "Grizzly Bears")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{daniels, daniels, daniels, daniels, bears}, nil)
	id := moveByName(t, e, 0, "Daniels, Temporal Agent", state.ZBattlefield)
	idx := putToLibAbilityIndex(t, e, id)
	addMana(t, e, 0, "") // a fresh priority decision after the move
	if opt, ok := findAbilityOption(e, id, idx); ok {
		t.Fatalf("the graveyard ability is offered from the battlefield: %+v", opt)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	before := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	addMana(t, e, 0, "11BG")
	opt, ok := findAbilityOption(e, id, idx)
	if !ok {
		t.Fatalf("the graveyard ability is not offered with {2}{B}{G} floating: %+v", e.Pending())
	}
	e.beginActivation(0, opt)
	if d := e.Pending(); d == nil || d.Kind != decision.KChoose || len(d.Options) != 1 || d.Options[0].Kind != "puttolibcost" {
		t.Fatalf("expected the put-to-library cost pick, got %+v", d)
	}
	submitChoices(t, e, e.Pending().Options[0].Index)
	if e.cast != nil {
		t.Fatalf("activation did not complete payment after the put-to-library pick: cast pending %+v", e.cast)
	}
	if got := e.G.Obj(id).Zone; got != state.ZLibrary {
		t.Fatalf("Daniels zone after the cost = %v, want library", got)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) != len(before)+1 || lib[len(lib)-1] != id {
		t.Fatalf("Daniels is not the bottom card of its owner's library: before %d cards, after %v", len(before), lib)
	}
	for i := range before {
		if lib[i] != before[i] {
			t.Fatalf("the library order changed at %d: %v, want %v (a shuffle is not a cost)", i, lib[:len(before)], before)
		}
	}
	if !hasEvent(e, events.MoveZone, id) || !hasEvent(e, events.AbilityPush, id) {
		t.Fatal("expected a MoveZone and an AbilityPush for the paid activation")
	}
	passUntilStackEmpty(t, e, 30)
	replayCheck(t, e, cfg)
}

// TestDanielsEntersReturnsCreatureCard pins the trigger side of the card
// untouched: entering returns a target creature card from its controller's
// graveyard to its owner's hand.
func TestDanielsEntersReturnsCreatureCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	daniels := lookup(t, reg, "Daniels, Temporal Agent")
	bears := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{daniels, bears}, nil)

	bearID := moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Grizzly Bears %d is not a graveyard card: %+v", bearID, o)
	}
	moveByName(t, e, 0, "Daniels, Temporal Agent", state.ZBattlefield)
	d := drainToTargetAsk(t, e, 40)
	if d.Kind != decision.KTarget {
		t.Fatalf("Daniels's enters trigger did not ask for its creature card target: %+v", d)
	}
	pick := indexOfObjOption(d, bearID)
	if pick < 0 {
		t.Fatalf("the target ask does not offer the graveyard Grizzly Bears %d: %+v", bearID, d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(bearID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("the returned creature card is not in its owner's hand: %+v", o)
	}
}
