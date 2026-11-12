package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const grantedBatchBlocker = "Name:Batch Blocker\nManaCost:G\nTypes:Creature Wall\nPT:0/1\nK:Reach\nOracle:Reach\n"

// bomberGrants activates Dropkick Bomber n times on the Goblin, on seat 0's third
// turn, and returns the Goblin.
func bomberGrants(t *testing.T, e *Engine, n int) state.ObjID {
	t.Helper()
	bomber := moveByName(t, e, 0, "Dropkick Bomber", state.ZBattlefield)
	goblin := moveByName(t, e, 0, "Goblin Piker", state.ZBattlefield)
	driveToStep(t, e, 3, 0, state.StepMain1)
	for range n {
		addMana(t, e, 0, "R")
		submitChoices(t, e, animateAbilityOption(t, e, bomber).Index)
		d := e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("no target decision after the bomber's ability (pending %+v)", d)
		}
		submitChoices(t, e, indexOfObjOption(d, goblin))
		settleActivation(t, e)
	}
	if grants := triggerGrantsOn(e, goblin); len(grants) != n {
		t.Fatalf("the goblin carries %d trigger grants, want %d", len(grants), n)
	}
	return goblin
}

func grantPushes(e *Engine, id state.ObjID) int {
	return countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.GrantTriggerPush && ev.Obj == id
	})
}

// Each grant is its own ability (CR 113.2c): three activations on one Goblin put
// three triggers on the stack from one combat damage event.
func TestDropkickBomberEachGrantTriggers(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := animateTriggersEngine(t, reg, lookup(t, reg, "Dropkick Bomber"), lookup(t, reg, "Goblin Piker"))
	goblin := bomberGrants(t, e, 3)

	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, goblin)
	passToKind(t, e, decision.KTriggerOrder)
	d := e.Pending()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 1, 2}}); err != nil {
		t.Fatal(err)
	}
	passUntilTurn(t, e, 4)
	if n := grantPushes(e, goblin); n != 3 {
		t.Fatalf("granted triggers pushed = %d, want 3", n)
	}
	if z := e.G.Obj(goblin).Zone; z != state.ZGraveyard {
		t.Fatalf("goblin zone = %v, want the graveyard", z)
	}
	replayCheck(t, e, cfg)
}

// "When this creature deals combat damage" is one trigger however many creatures
// the damage was split among: the event is the batch (CR 510.2, 603.2c).
func TestDropkickBomberGrantLatchesOneDamageBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := animateTriggersEngine(t, reg, lookup(t, reg, "Dropkick Bomber"), lookup(t, reg, "Goblin Piker"))
	goblin := bomberGrants(t, e, 1)
	walls := []state.ObjID{onBoardReady(t, e, 1, grantedBatchBlocker), onBoardReady(t, e, 1, grantedBatchBlocker)}

	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, goblin)
	passToKind(t, e, decision.KBlockers)
	submitBlockerPairsOnly(t, e, [2]state.ObjID{goblin, walls[0]}, [2]state.ObjID{goblin, walls[1]})
	passToKind(t, e, decision.KChoose)
	d := e.Pending()
	split := -1
	for _, o := range d.Options {
		if !strings.Contains(o.Label, "0 to") {
			split = o.Index
		}
	}
	if split < 0 {
		t.Fatalf("no division naming both walls: %+v", d.Options)
	}
	submitChoices(t, e, split)
	passUntilTurn(t, e, 4)
	for _, w := range walls {
		if z := e.G.Obj(w).Zone; z != state.ZGraveyard {
			t.Fatalf("wall %d zone = %v, want the graveyard: the damage was split", w, z)
		}
	}
	if n := grantPushes(e, goblin); n != 1 {
		t.Fatalf("granted triggers pushed = %d, want 1", n)
	}
}

// "This ability triggers only once each turn" holds on a line a static grants, as
// on a printed one (CR 603.2c): Hollowmurk Siege's Sultai mode draws one card for
// two placements in a turn, and draws again on the next turn.
func TestGrantedLineKeepsItsActivationLimit(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := animateTriggersEngine(t, reg, lookup(t, reg, "Hollowmurk Siege"), lookup(t, reg, "Goblin Piker"))
	moveByName(t, e, 0, "Hollowmurk Siege", state.ZHand)
	goblin := moveByName(t, e, 0, "Goblin Piker", state.ZBattlefield)
	driveToStep(t, e, 3, 0, state.StepMain1)
	siege := castFromHand(t, e, "Hollowmurk Siege", "BG")
	resolveStack(t, e)
	if d := e.Pending(); d != nil && d.Kind == decision.KModes {
		submitChoices(t, e, 0)
	}
	if got := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.ModeChosen && ev.Obj == siege && ev.Text == "Sultai"
	}); got != 1 {
		t.Fatalf("Sultai chosen %d times", got)
	}
	place := func(n int) {
		for range n {
			e.emit(events.Event{Kind: events.CounterChange, Obj: goblin, Counter: "P1P1", Amount: 2})
		}
		passOnce(t, e)
		resolveStack(t, e)
	}
	place(2)
	if n := grantPushes(e, siege); n != 1 {
		t.Fatalf("after two placements in a turn the grant triggered %d times, want 1", n)
	}
	passUntilTurn(t, e, 4)
	place(1)
	if n := grantPushes(e, siege); n != 2 {
		t.Fatalf("a placement on a later turn left %d triggers, want 2", n)
	}
	replayCheck(t, e, cfg)
}
