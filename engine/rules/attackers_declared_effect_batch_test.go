package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectAttackersDeclaredBatchFiresOnceOnSplitAttack: an Effect's
// "whenever you attack this turn" registration (Dalkovan Encampment's shape)
// is one triggering condition for the whole declaration (CR 508.1), like the
// printed batch line. A declaration split across two defenders emits two
// DeclareAttackers events; the registration fires on the first only.
func TestEffectAttackersDeclaredBatchFiresOnceOnSplitAttack(t *testing.T) {
	t.Parallel()
	const src = "Name:Signal Drum\nTypes:Artifact\n" +
		"A:AB$ Effect | Cost$ 0 | Triggers$ Beat | SpellDescription$ Beat the drum.\n" +
		"SVar:Beat:Mode$ AttackersDeclared | AttackingPlayer$ You | Execute$ Answer | TriggerZones$ Battlefield | TriggerDescription$ Whenever you attack this turn, gain 1 life.\n" +
		"SVar:Answer:DB$ GainLife | Defined$ You | LifeAmount$ 1\n" +
		"Oracle:x\n"
	e := threeSeatEngine(t)
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority, e.G.Turn = 0, 0, 1
	drum := onBoardReady(t, e, 0, src)
	a := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoardReady(t, e, 0, "Name:Fox\nManaCost:1 W\nTypes:Creature Fox\nPT:2/2\nOracle:x\n")
	activateAbilityOn(t, e, drum)
	passUntilStackEmpty(t, e, 20)

	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	ai := batchAttackerOptionIndex(t, e, a, 1)
	bi := batchAttackerOptionIndex(t, e, b, 2)
	declareAttackersAt(t, e, [][2]int{{int(a), ai}, {int(b), bi}})
	if len(e.G.Delayed) == 0 {
		t.Fatal("precondition failed: the Effect registered no trigger")
	}
	life := e.G.Players[0].Life
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life - life; got != 1 {
		t.Fatalf("the Effect's batch attack trigger gained %d life for one split declaration, want 1", got)
	}
}
