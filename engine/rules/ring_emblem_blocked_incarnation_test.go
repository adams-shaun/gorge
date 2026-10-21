package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCR70154cRingLevel3ReentryBeforeResolutionIsNotSacrificed pins CR 400.7
// for the Ring emblem's level 3: the promise names the blocking creature that
// existed when the Ring-bearer became blocked. A blocker that leaves and
// returns before the ability resolves is a NEW object (same ObjID, advanced
// Incarnation), so the ability must register no end-of-combat sacrifice.
func TestCR70154cRingLevel3ReentryBeforeResolutionIsNotSacrificed(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	blocker := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	temptByBirthdayEscape(t, e, reg, 0, 3)

	e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1,
		Pairs: [][2]state.ObjID{{bear, blocker}}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("level 3: becoming blocked queued %d abilities, want 1", len(e.G.Stack))
	}
	before := e.G.Obj(blocker).Incarnation

	// Flicker the blocker while the level-3 ability waits on the stack.
	e.emit(events.Event{Kind: events.MoveZone, Obj: blocker,
		From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.MoveZone, Obj: blocker,
		From: state.ZExile, To: state.ZBattlefield})
	if o := e.G.Obj(blocker); o.Zone != state.ZBattlefield || o.Incarnation == before {
		t.Fatalf("precondition: blocker zone %v incarnation %d, want battlefield with a moved incarnation from %d",
			o.Zone, o.Incarnation, before)
	}

	e.resolveTop()
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEndCombat})
	e.putTriggersOnStack()
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	if o := e.G.Obj(blocker); o.Zone != state.ZBattlefield {
		t.Fatalf("level 3: the returned blocker (zone %v) was sacrificed by a stale promise", o.Zone)
	}
	if n := countEvents(e, func(ev events.Event) bool {
		return events.IsSacrifice(ev) && ev.Obj == blocker
	}); n != 0 {
		t.Fatalf("level 3: the returned blocker was sacrificed %d times, want 0", n)
	}
}
