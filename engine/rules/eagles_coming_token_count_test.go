package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEaglesAreComingCreatesTokenPerReturnedCreature is the named acceptance
// for the delayed-registration Remembered$Amount defect: The Eagles Are
// Coming! kicked, returning two creatures, must mint one 4/4 Bird Soldier at
// the next upkeep for each returned creature. Its TrigToken body reads
// SVar:TokenX:Remembered$Amount against the registration's saved set, which
// trigger_delayed seeds into both Remembered and Captured.
func TestEaglesAreComingCreatesTokenPerReturnedCreature(t *testing.T) {
	eagles := lookup(t, testutil.CorpusRegistry(t), "The Eagles Are Coming!")
	bearA, bearB := card(t, eaglesBearASrc), card(t, eaglesBearBSrc)
	e, cfg := tokenReplGame(t, 11, eagles, bearA, bearB)
	a := moveSeededCard(t, e, 0, bearA, state.ZBattlefield)
	b := moveSeededCard(t, e, 0, bearB, state.ZBattlefield)
	spell := moveSeededCard(t, e, 0, eagles, state.ZHand)
	for _, id := range []state.ObjID{a, b} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: creature %d not on the battlefield before the cast: %+v", id, o)
		}
	}
	addMana(t, e, 0, "WWWWCC")
	submitChoices(t, e, castModeOption(t, e, spell, "kicked"))
	d := passUntilNonPriority(t, e, 30)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("precondition: the kicked cast did not ask for targets: %+v", d)
	}
	answerTargetAsk(t, e, []state.ObjID{a, b})
	passUntilStackEmpty(t, e, 30)

	// Precondition: the spell resolved and returned both creatures, and the
	// registration it made captured exactly those two (not an empty set).
	for _, id := range []state.ObjID{a, b} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
			t.Fatalf("precondition: creature %d was not returned to hand: %+v", id, o)
		}
	}
	registered := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedRegister && ev.Counter == "TrigToken" {
			registered++
			if len(ev.IDs) != 2 || ev.IDs[0] != a || ev.IDs[1] != b {
				t.Fatalf("registration capture = %v, want the two returned creatures [%d %d]", ev.IDs, a, b)
			}
		}
	}
	if registered != 1 {
		t.Fatalf("Eagles emitted %d TrigToken registrations, want exactly 1", registered)
	}

	// Drive past seat 1's upkeep (the next upkeep after the cast) and let
	// the delayed trigger resolve.
	driveToStep(t, e, 2, 1, state.StepDraw)
	passUntilStackEmpty(t, e, 30)
	pushed := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedPush && ev.Counter == "TrigToken" {
			pushed++
		}
	}
	if pushed != 1 {
		t.Fatalf("Eagles fired %d TrigToken delayed triggers, want exactly 1", pushed)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Bird Soldier Token"); got != 2 {
		t.Fatalf("Bird Soldier tokens = %d, want 2", got)
	}
	replayCheck(t, e, cfg)
}
