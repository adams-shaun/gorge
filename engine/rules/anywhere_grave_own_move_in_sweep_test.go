package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAnywhereGraveOwnMoveSilentInSweep pins CR 603.6c/603.10/113.6 for the
// from-anywhere spelling: "whenever a card is put into your graveyard from
// anywhere" is never a leaves-the-battlefield ability, so it is checked after
// the whole simultaneous departure, when the permanent carrying it is already
// in the graveyard and its battlefield ability is no longer functional. One
// DestroyAll takes the watcher (owned by seat 0) and a second creature (owned
// by seat 1); the other creature's card is not the watcher's, but the watcher's
// own card is, so the live walk used to read the watcher's LKI and queue its
// trigger for its own move. A true dies trigger (Origin$ Battlefield) keeps
// firing for its own death (CR 603.10a); the sibling shape is covered by the
// existing batch look-back tests.
func TestAnywhereGraveOwnMoveSilentInSweep(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	const witness = "Name:Anywhere grave witness\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Graveyard | ValidCard$ Card.YouOwn | Execute$ Heal\n" +
		"SVar:Heal:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n"
	watcher := onBoard(t, e, 0, witness)
	other := onBoard(t, e, 1, "Name:Other creature\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	var land state.ObjID
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Owner == 0 && o.Face() != nil && o.Face().Name == "Mountain" {
			land = o.ID
			break
		}
	}
	if land == 0 {
		t.Fatal("precondition: no Mountain to source the DestroyAll")
	}
	effects.Resolve(e, &effects.Ctx{Source: land, Controller: 0},
		&cards.SA{Kind: "SP", API: "DestroyAll", Params: map[string]string{"ValidCards": "Creature"}})
	if got := e.G.Obj(watcher).Zone; got != state.ZGraveyard {
		t.Fatalf("precondition: the watcher is in %s after DestroyAll, want graveyard", got)
	}
	if got := e.G.Obj(other).Zone; got != state.ZGraveyard {
		t.Fatalf("precondition: the second creature is in %s after DestroyAll, want graveyard", got)
	}
	queued := 0
	for i := range e.pendingTriggers {
		if e.pendingTriggers[i].Source == watcher {
			queued++
		}
	}
	if queued != 0 {
		t.Fatalf("the from-anywhere watcher queued %d triggers for its own move in the sweep, want 0 (CR 603.6c/603.10: not a leaves-the-battlefield ability, and its battlefield ability is gone)", queued)
	}
}

// TestAnywhereGraveSelfLineStillFiresForItsOwnMove is the sibling guard for
// the own-move gate: a line that names itself ("when CARDNAME is put into a
// graveyard from anywhere", ValidCard$ Card.Self, no TriggerZones$) is a
// self-move trigger that looks back and must keep firing for its own death
// (Ulamog, the Infinite Gyre; zoneGate's self ChangesZone admission). Only the
// general watcher shape is silent.
func TestAnywhereGraveSelfLineStillFiresForItsOwnMove(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	const selfLine = "Name:Anywhere grave self\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ Heal\n" +
		"SVar:Heal:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n"
	id := onBoard(t, e, 0, selfLine)
	var land state.ObjID
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Owner == 0 && o.Face() != nil && o.Face().Name == "Mountain" {
			land = o.ID
			break
		}
	}
	if land == 0 {
		t.Fatal("precondition: no Mountain to source the DestroyAll")
	}
	effects.Resolve(e, &effects.Ctx{Source: land, Controller: 0},
		&cards.SA{Kind: "SP", API: "DestroyAll", Params: map[string]string{"ValidCards": "Creature"}})
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("precondition: the self line's creature is in %s after DestroyAll, want graveyard", got)
	}
	queued := 0
	for i := range e.pendingTriggers {
		if e.pendingTriggers[i].Source == id {
			queued++
		}
	}
	if queued != 1 {
		t.Fatalf("the self from-anywhere line queued %d triggers for its own move, want 1 (CR 603.10a self-move look-back)", queued)
	}
}
