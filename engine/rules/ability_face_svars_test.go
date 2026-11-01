package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	twoFacedSrc = "Name:Morning Face\nTypes:Creature Human\nPT:1/1\n" +
		"A:AB$ GainLife | Cost$ 0 | LifeAmount$ X | SpellDescription$ Gain X life.\nSVar:X:Number$3\n" +
		"AlternateMode:DoubleFaced\nOracle:x\nALTERNATE\n" +
		"Name:Evening Face\nTypes:Creature Wolf\nPT:2/2\n" +
		"A:AB$ GainLife | Cost$ Sac<1/CARDNAME> | LifeAmount$ Y | SpellDescription$ Gain Y life.\nSVar:Y:Number$5\nOracle:x\n"
	sacModelSrc = "Name:Sac Model\nTypes:Creature Wolf\nPT:2/2\n" +
		"A:AB$ GainLife | Cost$ Sac<1/CARDNAME> | LifeAmount$ Y | SpellDescription$ Gain Y life.\nSVar:Y:Number$5\nOracle:x\n"
	diesModelSrc = "Name:Dies Model\nTypes:Creature Wolf\nPT:2/2\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ TrigGain | TriggerDescription$ When this dies, gain Y life.\n" +
		"SVar:TrigGain:DB$ GainLife | LifeAmount$ Y\nSVar:Y:Number$5\nOracle:x\n"
	promiseModelSrc = "Name:Promise Model\nTypes:Creature Wolf\nPT:2/2\n" +
		"A:AB$ DelayedTrigger | Cost$ Sac<1/CARDNAME> | Mode$ Phase | Phase$ End of Turn | Execute$ TrigPromise | SpellDescription$ At the next end step, gain 5 life.\n" +
		"SVar:TrigPromise:DB$ GainLife | LifeAmount$ 5\nOracle:x\n"
	extraTurnModelSrc = "Name:Extra Turn Model\nTypes:Creature Wolf\nPT:2/2\n" +
		"A:AB$ AddTurn | Cost$ Sac<1/CARDNAME> | NumTurns$ 1 | ExtraTurnDelayedTrigger$ Later | ExtraTurnDelayedTriggerExecute$ TrigPromise | SpellDescription$ Take an extra turn. At its end step, gain 5 life.\n" +
		"SVar:Later:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn\nSVar:TrigPromise:DB$ GainLife | LifeAmount$ 5\nOracle:x\n"
	extraCombatModelSrc = "Name:Extra Combat Model\nTypes:Creature Wolf\nPT:2/2\n" +
		"A:AB$ AddPhase | Cost$ Sac<1/CARDNAME> | ExtraPhase$ Combat | AfterPhase$ EndCombat | ExtraPhaseDelayedTrigger$ Later | ExtraPhaseDelayedTriggerExcute$ TrigPromise | SpellDescription$ An extra combat. At its start, gain 5 life.\n" +
		"SVar:Later:DB$ DelayedTrigger | Mode$ Phase | Phase$ BeginCombat\nSVar:TrigPromise:DB$ GainLife | LifeAmount$ 5\nOracle:x\n"
	blankBearSrc = "Name:Blank Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

func activateAbilityOn(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.priorityRound()
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == id {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no ability offered on %d", id)
}

// copyOnBoard puts a Blank Bear on seat 0's battlefield as a copy of model, so
// leaving the battlefield takes the copied SVars off its current face.
func copyOnBoard(t *testing.T, e *Engine, model string) state.ObjID {
	t.Helper()
	id := onBoard(t, e, 0, blankBearSrc)
	e.G.Obj(id).SummonSick = false
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: id, IDs: []state.ObjID{onBoard(t, e, 1, model)}})
	return id
}

func wantGained(t *testing.T, e *Engine, before, want int32) {
	t.Helper()
	if got := e.G.Players[0].Life - before; got != want {
		t.Fatalf("gained %d life, want %d", got, want)
	}
}

// TestActivatedAbilityResolvesWithItsFacesSVars pins CR 113.7a: an ability on
// the stack keeps the SVars of the face it was activated from, whatever its
// source is by the time it resolves.
func TestActivatedAbilityResolvesWithItsFacesSVars(t *testing.T) {
	t.Parallel()
	t.Run("a back face sacrificed as the cost", func(t *testing.T) {
		t.Parallel()
		e := handEngine(t)
		id := onBoard(t, e, 0, twoFacedSrc)
		e.G.Obj(id).SummonSick = false
		e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1, Text: "Transformed"})
		before := e.G.Players[0].Life
		activateAbilityOn(t, e, id)
		passUntilStackEmpty(t, e, 20)
		wantGained(t, e, before, 5)
	})
	t.Run("a front face transformed in response", func(t *testing.T) {
		t.Parallel()
		e := handEngine(t)
		id := onBoard(t, e, 0, twoFacedSrc)
		e.G.Obj(id).SummonSick = false
		before := e.G.Players[0].Life
		activateAbilityOn(t, e, id)
		e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: 1, Text: "Transformed"})
		passUntilStackEmpty(t, e, 20)
		wantGained(t, e, before, 3)
	})
	t.Run("a copy sacrificed as the cost", func(t *testing.T) {
		t.Parallel()
		e := handEngine(t)
		id := copyOnBoard(t, e, sacModelSrc)
		before := e.G.Players[0].Life
		activateAbilityOn(t, e, id)
		passUntilStackEmpty(t, e, 20)
		wantGained(t, e, before, 5)
	})
}

// TestDepartedCopyTriggersOnLeaving pins CR 603.10a: a permanent that was a
// copy as it left the battlefield looks back at what it copied.
func TestDepartedCopyTriggersOnLeaving(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	id := copyOnBoard(t, e, diesModelSrc)
	e.priorityRound()
	before := e.G.Players[0].Life
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)
	wantGained(t, e, before, 5)
}

// TestDepartedCopyKeepsItsPromises pins CR 603.7: a delayed trigger made by a
// copy fires though its source has stopped being one, whichever path registers
// it.
func TestDepartedCopyKeepsItsPromises(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, model string
		turn        int32
		step        state.Step
	}{
		{"an end step promise", promiseModelSrc, 1, state.StepEnd},
		{"an extra turn's rider", extraTurnModelSrc, 2, state.StepEnd},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t)
			id := copyOnBoard(t, e, tc.model)
			before := e.G.Players[0].Life
			activateAbilityOn(t, e, id)
			passUntilStackEmpty(t, e, 20)
			wantGained(t, e, before, 0)
			driveToStepAll(t, e, tc.turn, 0, tc.step)
			passUntilStackEmpty(t, e, 20)
			wantGained(t, e, before, 5)
		})
	}
	t.Run("an extra combat's rider", func(t *testing.T) {
		t.Parallel()
		e := handEngine(t)
		id := copyOnBoard(t, e, extraCombatModelSrc)
		before := e.G.Players[0].Life
		activateAbilityOn(t, e, id)
		passUntilStackEmpty(t, e, 20)
		for i := 0; i < 40 && e.G.Players[0].Life == before && e.G.Step != state.StepMain2; i++ {
			passOnceP(t, e)
		}
		wantGained(t, e, before, 5)
	})
}
