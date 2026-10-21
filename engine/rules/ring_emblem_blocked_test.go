package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.54c, the Ring emblem's level 3: "Whenever your Ring-bearer becomes
// blocked by a creature, the blocking creature's controller sacrifices it at
// end of combat." Tempted by the real corpus Birthday Escape
// (temptByBirthdayEscape, ring_emblem_cr_test.go).

// TestCR70154cRingLevel3BlockerSacrificedAtEndOfCombat: at three temptations
// the Ring-bearer becoming blocked asks nothing, keeps the blocker on the
// battlefield through combat, and the blocker's controller sacrifices it when
// the end of combat step begins.
func TestCR70154cRingLevel3BlockerSacrificedAtEndOfCombat(t *testing.T) {
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
	e.resolveTop()
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("level 3: resolving the blocked trigger posed %v, want none", d.Kind)
	}
	if o := e.G.Obj(blocker); o.Zone != state.ZBattlefield {
		t.Fatalf("level 3: the blocker left before end of combat (zone %v)", o.Zone)
	}

	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEndCombat})
	e.putTriggersOnStack()
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	if o := e.G.Obj(blocker); o.Zone != state.ZGraveyard {
		t.Fatalf("level 3: blocker zone %v at end of combat, want graveyard (sacrificed)", o.Zone)
	}
	if countEvents(e, func(ev events.Event) bool { return events.IsSacrifice(ev) && ev.Obj == blocker }) != 1 {
		t.Fatal("level 3: the blocker was not SACRIFICED")
	}
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield {
		t.Fatalf("level 3: the Ring-bearer left the battlefield (zone %v)", o.Zone)
	}
}

// TestCR70154cRingLevel3EachBlockerTriggersSeparately: "becomes blocked BY A
// CREATURE" triggers once per blocking creature, so two blockers queue two
// emblem abilities, and both blockers are sacrificed at end of combat while
// a creature of the same player that blocked a different attacker is not.
func TestCR70154cRingLevel3EachBlockerTriggersSeparately(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	b1 := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	b2 := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	temptByBirthdayEscape(t, e, reg, 0, 3)
	other := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	b3 := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))

	e.emit(events.Event{Kind: events.DeclareBlockers, Player: 1,
		Pairs: [][2]state.ObjID{{bear, b1}, {bear, b2}, {other, b3}}})
	e.putTriggersOnStack()
	answerTriggerOrders(t, e) // two simultaneous emblem abilities of seat 0
	if len(e.G.Stack) != 2 {
		t.Fatalf("level 3: two blockers queued %d abilities, want 2", len(e.G.Stack))
	}
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEndCombat})
	e.putTriggersOnStack()
	answerTriggerOrders(t, e) // the two delayed sacrifices
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	for _, id := range []state.ObjID{b1, b2} {
		if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
			t.Fatalf("level 3: blocker %d zone %v at end of combat, want graveyard", id, o.Zone)
		}
	}
	if o := e.G.Obj(b3); o.Zone != state.ZBattlefield {
		t.Fatalf("level 3: the creature that blocked a non-bearer left the battlefield (zone %v)", o.Zone)
	}
}
