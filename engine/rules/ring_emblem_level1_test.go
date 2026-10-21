package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 701.54c, the Ring emblem's first ability, which every emblem has from
// the first temptation: "Your Ring-bearer is legendary and can't be blocked
// by creatures with greater power." Tempted by the real corpus Birthday
// Escape (temptByBirthdayEscape, ring_emblem_cr_test.go).

// TestCR70154cRingLevel1BearerIsLegendary: one temptation makes the
// Ring-bearer legendary in its derived types and to a Creature.Legendary
// filter; a creature of the same player that is not the bearer, and the
// opponent's creature, stay non-legendary.
func TestCR70154cRingLevel1BearerIsLegendary(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	theirs := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	legendary := func(id state.ObjID) bool {
		return slices.Contains(e.typeCharacteristics(id, 0), "Legendary") &&
			e.matchesSpecFrom("Creature.Legendary", id, 0, 0)
	}
	if legendary(bear) {
		t.Fatal("fixture: Grizzly Bears is legendary before any temptation")
	}

	temptByBirthdayEscape(t, e, reg, 0, 1)
	if e.G.Players[0].RingBearer != bear {
		t.Fatalf("fixture: Ring-bearer = %d, want the bear %d", e.G.Players[0].RingBearer, bear)
	}
	if !legendary(bear) {
		t.Fatalf("level 1: the Ring-bearer's derived types %v lack Legendary", e.typeCharacteristics(bear, 0))
	}
	other := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	if legendary(other) {
		t.Fatal("level 1: a creature that is not the Ring-bearer became legendary")
	}
	if legendary(theirs) {
		t.Fatal("level 1: the opponent's creature became legendary")
	}
}

// TestCR70154cRingLevel1GreaterPowerCantBlockBearer: after one temptation a
// 3-power creature can no longer block the 2-power Ring-bearer, an equal-power
// creature still can, and the 3-power creature can still block a creature of
// the same player that is not the bearer.
func TestCR70154cRingLevel1GreaterPowerCantBlockBearer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	bear := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	giant := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Hill Giant"))
	equal := onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Grizzly Bears"))
	// The bear attacks seat 1 (canBlock reads the attack); with one
	// temptation no emblem ability triggers on it.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	if !e.canBlock(giant, bear) || !e.canBlock(equal, bear) {
		t.Fatal("fixture: before any temptation both creatures can block the bear")
	}

	temptByBirthdayEscape(t, e, reg, 0, 1)
	if e.canBlock(giant, bear) {
		t.Fatal("level 1: a creature with greater power can block the Ring-bearer")
	}
	if !e.canBlock(equal, bear) {
		t.Fatal("level 1: a creature with equal power can no longer block the Ring-bearer")
	}
	other := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Grizzly Bears"))
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{other}})
	if !e.canBlock(giant, other) {
		t.Fatal("level 1: the restriction reached a creature that is not the Ring-bearer")
	}
}

// TestCR70154cRingLevel1LegendaryFollowsTheBearer: the emblem's type grant
// is registered once (when the emblem is created, before the first bearer
// choice is answered) and follows the live designation -- answering a later
// temptation with a different creature moves Legendary to it and off the
// old bearer -- and the game still replays from its log.
func TestCR70154cRingLevel1LegendaryFollowsTheBearer(t *testing.T) {
	t.Parallel()
	e, cfg, bear, gorilla := kr3RingBoard(t, 93)
	d := kr3Cast(t, e, "Ring Sorcery", "B", -1)
	if d == nil || d.ResumeKind != "ring_bearer" {
		t.Fatalf("decision = %+v, want a ring_bearer ask", d)
	}
	kr3Answer(t, e, kr3OptionObj(t, d, gorilla))
	registered := 0
	for i := range e.continuous {
		if e.continuous[i].Affects == "Card.IsRingbearer+YouCtrl" {
			registered++
		}
	}
	if registered != 1 {
		t.Fatalf("the emblem's type grant is registered %d times, want 1", registered)
	}
	if !legendaryUnderLayers(e, gorilla) || legendaryUnderLayers(e, bear) {
		t.Fatalf("after choosing the gorilla: legendary gorilla=%v bear=%v, want true/false",
			legendaryUnderLayers(e, gorilla), legendaryUnderLayers(e, bear))
	}

	// Return the resolved sorcery to hand with a logged move and tempt again,
	// this time choosing the bear.
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZGraveyard, 0)...) {
		if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Ring Sorcery" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZHand})
		}
	}
	d = kr3Cast(t, e, "Ring Sorcery", "B", -1)
	if d == nil || d.ResumeKind != "ring_bearer" {
		t.Fatalf("second decision = %+v, want a ring_bearer ask", d)
	}
	kr3Answer(t, e, kr3OptionObj(t, d, bear))
	if legendaryUnderLayers(e, gorilla) || !legendaryUnderLayers(e, bear) {
		t.Fatalf("after choosing the bear: legendary gorilla=%v bear=%v, want false/true",
			legendaryUnderLayers(e, gorilla), legendaryUnderLayers(e, bear))
	}
	if e.matchesSpecFrom("Creature.Legendary", gorilla, 0, 0) || !e.matchesSpecFrom("Creature.Legendary", bear, 0, 0) {
		t.Fatal("after choosing the bear: the Creature.Legendary filter did not follow the bearer")
	}
	replayCheck(t, e, cfg)
}
