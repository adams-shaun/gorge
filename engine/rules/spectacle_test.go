package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func spectacleOption(t *testing.T, e *Engine, id state.ObjID) *decision.Option {
	t.Helper()
	var found *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Obj == id && o.Mode == altMode(altSpectacle) {
			o := o
			found = &o
		}
	}
	return found
}

// CR 702.137a: the spectacle cost is offered only while an opponent of the
// caster has lost life this turn.
func TestSpectacleNeedsAnOpponentToHaveLostLife(t *testing.T) {
	t.Parallel()
	src := "Name:Spec Bolt\nManaCost:2 R\nTypes:Instant\nK:Spectacle:R\n" +
		"A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n"
	e, cfg, bolt := newFixtureDeck(t, 31, src)
	addMana(t, e, 0, "R")

	if got := e.G.Players[1].LifeLostThisTurn; got != 0 {
		t.Fatalf("precondition: seat 1 lost %d life before the test", got)
	}
	if o := e.G.Obj(bolt); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Spec Bolt not in hand: %+v", o)
	}
	if spectacleOption(t, e, bolt) != nil {
		t.Fatal("spectacle offered before any opponent lost life")
	}

	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -1})
	e.pending = nil
	e.Advance()
	if e.G.Players[0].LifeLostThisTurn != 1 || e.G.Players[1].LifeLostThisTurn != 0 {
		t.Fatalf("precondition: life lost this turn = %d/%d, want 1/0",
			e.G.Players[0].LifeLostThisTurn, e.G.Players[1].LifeLostThisTurn)
	}
	if spectacleOption(t, e, bolt) != nil {
		t.Fatal("spectacle offered after only the caster lost life")
	}

	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	e.pending = nil
	e.Advance()
	if e.G.Players[1].LifeLostThisTurn != 1 {
		t.Fatalf("precondition: seat 1 LifeLostThisTurn = %d, want 1", e.G.Players[1].LifeLostThisTurn)
	}
	opt := spectacleOption(t, e, bolt)
	if opt == nil {
		t.Fatal("spectacle not offered after an opponent lost life")
	}
	pool := e.G.Players[0].Pool.Total()
	submitChoices(t, e, opt.Index)
	castObj2 := e.Pending()
	if castObj2 != nil && castObj2.Kind != decision.KPriority {
		submitChoices(t, e, 0) // the damage target
	}
	if got := e.G.Players[0].Pool.Total(); got != pool-1 {
		t.Fatalf("pool %d -> %d, want exactly {R} spent", pool, got)
	}
	if o := e.G.Obj(bolt); o.Zone != state.ZStack && o.Zone != state.ZGraveyard {
		t.Fatalf("Spec Bolt in %s after the spectacle cast", o.Zone)
	}
	e.Advance()
	for e.Pending() != nil && e.Pending().Kind == decision.KPriority && len(e.G.Stack) > 0 {
		submitChoices(t, e, 0)
	}
	if o := e.G.Obj(bolt); o.Zone != state.ZGraveyard {
		t.Fatalf("Spec Bolt did not resolve: zone %s", o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestSpectacleCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Blade Juggler", "Body Count", "Light Up the Stage"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
