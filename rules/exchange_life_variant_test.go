package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func readyExchangeCard(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
		t.Fatalf("exchange source precondition: %+v", e.G.Obj(id))
	}
	// The real activation is a tap ability; give the permanent a turn before
	// asking priority so summoning sickness cannot make this test vacuous.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	e.pending = nil
	e.priorityRound()
}

func activateExchange(t *testing.T, e *Engine, source state.ObjID, target state.PlayerID) {
	t.Helper()
	o := e.G.Obj(source)
	if o == nil || o.Face() == nil || len(o.Face().Abilities) == 0 || o.Face().Abilities[0].API != "ExchangeLifeVariant" {
		t.Fatalf("source has no ExchangeLifeVariant ability: %+v", o)
	}
	opt := abilityOption(t, e, source, 0)
	submitChoices(t, e, opt.Index)
	if target != 0 {
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("target decision = %+v", d)
		}
		idx := indexOfPlayerOption(d, target)
		if idx < 0 {
			t.Fatalf("target %d absent from %+v", target, d.Options)
		}
		submitChoices(t, e, idx)
	}
	passUntilStackEmpty(t, e, 40)
}

func TestTreeOfPerditionExchangesTargetLifeAndToughness(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := searchEngine(t, reg, "Tree of Perdition")
	tree := searchMoveByName(t, e, "Tree of Perdition", state.ZBattlefield)
	readyExchangeCard(t, e, tree)
	// Make the target's old life and the Tree's characteristic visibly distinct.
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -16})
	before := e.Toughness(tree)
	oldLife := e.G.Players[1].Life
	if before == oldLife || before <= 0 {
		t.Fatalf("precondition values: Tree toughness %d, opponent life %d", before, oldLife)
	}
	activateExchange(t, e, tree, 1)
	if got := e.G.Players[1].Life; got != before {
		t.Fatalf("opponent life %d, want old toughness %d", got, before)
	}
	if got := e.Toughness(tree); got != oldLife {
		t.Fatalf("Tree toughness %d, want old life %d", got, oldLife)
	}
	if e.Power(tree) != 0 {
		t.Fatalf("Tree power %d changed during toughness exchange", e.Power(tree))
	}
	// Mark damage against the newly lowered, real derived toughness and drive
	// the ordinary SBA boundary.
	e.emit(events.Event{Kind: events.Damage, Obj: tree, Amount: 5})
	e.checkStateBased()
	if e.G.Obj(tree).Zone != state.ZGraveyard {
		t.Fatalf("Tree zone %s, want Graveyard after marked damage and lowered toughness", e.G.Obj(tree).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestTreeOfRedemptionExchangesControllerLifeAndToughness(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := searchEngine(t, reg, "Tree of Redemption")
	tree := searchMoveByName(t, e, "Tree of Redemption", state.ZBattlefield)
	readyExchangeCard(t, e, tree)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -16})
	before := e.Toughness(tree)
	oldLife := e.G.Players[0].Life
	if before == oldLife || before <= 0 {
		t.Fatalf("precondition values: Tree toughness %d, controller life %d", before, oldLife)
	}
	activateExchange(t, e, tree, 0)
	if e.G.Players[0].Life != before || e.Toughness(tree) != oldLife || e.Power(tree) != 0 {
		t.Fatalf("redemption exchange: life=%d P/T=%d/%d, want %d %d/%d", e.G.Players[0].Life, e.Power(tree), e.Toughness(tree), before, 0, oldLife)
	}
	replayCheck(t, e, cfg)
}

func TestEvraExchangesControllerLifeAndPower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := searchEngine(t, reg, "Evra, Halcyon Witness")
	evra := searchMoveByName(t, e, "Evra, Halcyon Witness", state.ZBattlefield)
	readyExchangeCard(t, e, evra)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 6})
	beforePower, beforeToughness := e.Power(evra), e.Toughness(evra)
	oldLife := e.G.Players[0].Life
	if beforePower == oldLife || beforePower <= 0 || beforeToughness <= 0 {
		t.Fatalf("precondition values: Evra P/T=%d/%d, life=%d", beforePower, beforeToughness, oldLife)
	}
	addMana(t, e, 0, "CCCC")
	activateExchange(t, e, evra, 0)
	if e.G.Players[0].Life != beforePower || e.Power(evra) != oldLife || e.Toughness(evra) != beforeToughness {
		t.Fatalf("Evra exchange: life=%d P/T=%d/%d, want %d %d/%d", e.G.Players[0].Life, e.Power(evra), e.Toughness(evra), beforePower, oldLife, beforeToughness)
	}
	replayCheck(t, e, cfg)
}
