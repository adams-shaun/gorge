package rules

// graveyard_leave_lookback_test.go pins CR 603.10a for the "leaves your
// graveyard" trigger family: those triggers look back in time and judge the
// card as it last existed in the graveyard. A card returned from a graveyard
// as a non-creature permanent must still satisfy a watcher's
// ValidCard$ Creature.YouOwn, because in the graveyard it was a creature card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gyLeaveWatcher is Along the Crooked Way's printed second trigger: a
// battlefield watcher over creature cards leaving its controller's graveyard.
const gyLeaveWatcher = "Name:Graveyard Watcher\nManaCost:2 B\nTypes:Enchantment\n" +
	"T:Mode$ ChangesZone | Origin$ Graveyard | Destination$ Any | TriggerZones$ Battlefield | ValidCard$ Creature.YouOwn | Execute$ TrigGain\n" +
	"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n"

// gyLeaveMover is the creature card used by the positive control below.

const gyLeaveMover = "Name:Returned Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// TestGraveyardLeaveTriggerPositiveControlIsCreatureToExile proves the setup's
// watcher fires when the moved card keeps its creature card type, so the
// look-back pin above is not passing because the watcher is dead.
func TestGraveyardLeaveTriggerPositiveControlIsCreatureToExile(t *testing.T) {
	t.Parallel()
	e := newSeats(t, 2)
	e.pending = nil
	watcher := onBoard(t, e, 0, gyLeaveWatcher)
	mover := placeInZone(t, e, 0, state.ZGraveyard, card(t, gyLeaveMover))
	if !hasTypeWord(card(t, gyLeaveMover).Faces[0].Types, "Creature") {
		t.Fatal("precondition: mover fixture is not a creature card")
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: mover, From: state.ZGraveyard, To: state.ZExile, Player: 0})
	if got := len(e.pendingTriggers); got != 1 || e.pendingTriggers[0].Source != watcher {
		t.Fatalf("queued %+v, want the watcher's trigger", e.pendingTriggers)
	}
}

// tomReturnFixture is Tom, Bert, and William's printed die trigger: return the
// card to the battlefield and animate it into an artifact (no longer a
// creature).
const tomReturnFixture = "Name:Returning Troll\nManaCost:3 B G\nTypes:Legendary Creature Troll\nPT:5/5\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+Creature | Execute$ ReturnAnimated\n" +
	"SVar:ReturnAnimated:DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | StaticEffect$ AnimateArtifact | GainControl$ True\n" +
	"SVar:AnimateArtifact:Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Artifact | RemoveCardTypes$ True\nOracle:x\n"

// TestGraveyardLeaveTriggerFiresOnAnimatedReturn drives the real ChangeZone
// return path (the StaticEffect$ AnimateArtifact rider) rather than a bare MoveZone:
// the returning permanent is no longer a creature, but the watcher must see
// the card as it last existed in the graveyard (CR 603.10a).
func TestGraveyardLeaveTriggerFiresOnAnimatedReturn(t *testing.T) {
	t.Parallel()
	e := newSeats(t, 2)
	e.pending = nil
	watcher := onBoard(t, e, 0, gyLeaveWatcher)
	tom := onBoard(t, e, 0, tomReturnFixture)
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: tom, From: state.ZBattlefield,
		To: state.ZGraveyard, Text: "lethal damage"})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("precondition: the return trigger did not queue: %+v", e.pendingTriggers)
	}
	if e.putTriggersOnStack() || len(e.G.Stack) != 1 {
		t.Fatalf("precondition: return trigger not on the stack: %v", e.G.Stack)
	}
	e.resolveTop()

	// Preconditions: the card returned as an artifact (so the live read cannot
	// match Creature), and the watcher is still on the battlefield.
	if o := e.G.Obj(tom); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Tom did not return: %+v", o)
	}
	if hasTypeWord(e.Derived(tom).Types, "Creature") {
		t.Fatalf("precondition: returned Tom is still a creature: %v", e.Derived(tom).Types)
	}
	if o := e.G.Obj(watcher); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: watcher left the battlefield: %+v", o)
	}
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != watcher {
		t.Fatalf("queued %+v, want the watcher's leaves-the-graveyard trigger", e.pendingTriggers)
	}
}
