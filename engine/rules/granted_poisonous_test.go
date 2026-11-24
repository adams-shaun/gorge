package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestVirulentSliverGrantsPoisonous pins CR 702.70a/b for a GRANTED
// poisonous: Virulent Sliver's "All Sliver creatures have poisonous 1"
// (layer 6) reaches itself and a Spined Sliver beside it. Both attack
// unblocked; after combat damage each Sliver's own poisonous trigger is on the
// stack (two instances), and once they resolve the defender has 2 POISON. A
// log-only replay rebuilds the identical game.
func TestVirulentSliverGrantsPoisonous(t *testing.T) {
	t.Parallel()
	e, cfg := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Virulent Sliver", "Spined Sliver"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	virulent := findBattlefield(t, e, 0, "Virulent Sliver", 0)
	spined := findBattlefield(t, e, 0, "Spined Sliver", 0)
	for _, id := range []state.ObjID{virulent, spined} {
		if !e.HasKeyword(id, "Poisonous") {
			t.Fatalf("precondition: %s lacks the granted Poisonous", e.G.Obj(id).Face().Name)
		}
		if e.G.Obj(id).Face().HasKeyword("Poisonous") {
			t.Fatalf("precondition: %s prints Poisonous; not the granted path", e.G.Obj(id).Face().Name)
		}
	}
	if got := e.G.Players[1].Counter("POISON"); got != 0 {
		t.Fatalf("precondition: defender starts with %d poison", got)
	}
	e.askAttackers()
	submitAttackers(t, e, virulent, spined)
	// No blockers: p1 has no creatures, so the blockers step either asks for
	// an empty declaration or skips it.
	if d := e.Pending(); d != nil && d.Kind == decision.KBlockers {
		submitBlockersOnly(t, e)
		drainCombatPriority(t, e)
	}
	drainCombatDamageToStack(t, e)
	if d := e.Pending(); d != nil && d.Kind == decision.KTriggerOrder {
		choices := make([]int, len(d.Options))
		for i, o := range d.Options {
			choices[i] = o.Index
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("precondition: defender life = %d, want 17 (an unblocked 1/1 and 2/2)", got)
	}
	if got := len(e.G.Stack); got != 2 {
		t.Fatalf("stack after combat damage holds %d objects, want 2 poisonous triggers (one per Sliver)", got)
	}
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Counter("POISON"); got != 2 {
		t.Fatalf("defender poison = %d, want 2 (poisonous 1 from each Sliver)", got)
	}
	replayCheck(t, e, cfg)
}
