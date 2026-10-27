package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSpawnerBitingPalmNinjaExilesFromDamagedPlayer drives the real card: the
// Ninja's immediate trigger body reads `Defined$`/`DefinedPlayer$
// Spawner>TriggeredTarget`, which must name the damaged player (seat 1), not
// the Ninja's controller. Before the definedSpec chain arm the reveal came
// from seat 0 and the exile refused the spelling with a Note.
func TestSpawnerBitingPalmNinjaExilesFromDamagedPlayer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	bystander := card(t, "Name:Seat One Bystander\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg, ids := dsBoardWith(t, reg, bystander, "Biting-Palm Ninja")
	ninja := ids["Biting-Palm Ninja"]
	cardToHandSeat(t, e, 1, bystander)
	var theirs state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 1) {
		if e.G.Obj(id).Card == bystander {
			theirs = id
		}
	}
	if theirs == 0 {
		t.Fatal("precondition: seat 1's nonland card is not in hand")
	}
	if got := e.G.Obj(ninja).Counter("Menace"); got != 1 {
		t.Fatalf("precondition: menace counters = %d, want 1", got)
	}
	handBefore := len(e.G.Zone(state.ZHand, 0))

	e.Advance()
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, ninja)

	exiled := false
	for i := 0; i < 200 && !e.G.Over && !exiled; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KTriggerOptional:
			submitChoices(t, e, 0) // yes: remove the menace counter
		case decision.KPriority:
			passFirst(t, e)
		default:
			pick := -1
			for _, o := range d.Options {
				if o.Obj == theirs {
					pick = o.Index
				}
			}
			if pick < 0 {
				submitChoices(t, e, d.Options[0].Index)
				continue
			}
			submitChoices(t, e, pick)
		}
		exiled = e.G.Obj(theirs).Zone == state.ZExile
	}
	if hasNote(e, "cannot resolve the hand owner") {
		t.Fatal("the exile refused the Spawner>TriggeredTarget hand owner")
	}
	if z := e.G.Obj(theirs).Zone; z != state.ZExile {
		t.Fatalf("seat 1's nonland card is in %s, want exile", z)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore {
		t.Fatalf("seat 0 hand = %d, want %d", got, handBefore)
	}
	replayCheck(t, e, cfg)
}
