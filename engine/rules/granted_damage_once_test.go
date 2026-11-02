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
		submitChoices(t, e, optionForObj(d, goblin))
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
