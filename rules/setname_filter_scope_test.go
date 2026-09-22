package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// setNameScopeBoard puts a SetName$ Equipment and two creatures on the
// battlefield and returns the engine plus the three object ids.
func setNameScopeBoard(t *testing.T) (e *Engine, bladeID, bearID, elfID state.ObjID) {
	t.Helper()
	blade := card(t, "Name:First Blade\nManaCost:1\nTypes:Artifact Equipment\nK:Equip:1\n"+
		"S:Mode$ Continuous | Affected$ Creature.EquippedBy | SetName$ First Name | Description$ x\nOracle:x\n")
	bear := card(t, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	elf := card(t, "Name:Elvish Mystic\nManaCost:G\nTypes:Creature Elf Druid\nPT:1/1\nOracle:x\n")
	e, _, _ = corpusDeckEngine(t, nil, []*cards.Card{blade, bear, elf})
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if o.Zone != state.ZBattlefield {
			continue
		}
		switch o.Card {
		case blade:
			bladeID = o.ID
		case bear:
			bearID = o.ID
		case elf:
			elfID = o.ID
		}
	}
	if bladeID == 0 || bearID == 0 || elfID == 0 {
		t.Fatalf("setup: blade=%d bear=%d elf=%d", bladeID, bearID, elfID)
	}
	// Deliberately NOT attached: each test decides when applicability changes,
	// so a clone taken before the change can be shown not to follow it.
	return e, bladeID, bearID, elfID
}

// equip attaches the Equipment and checks the attachment took.
func equip(t *testing.T, e *Engine, bladeID, toID state.ObjID) {
	t.Helper()
	e.emit(events.Event{Kind: events.Attach, Obj: bladeID, IDs: []state.ObjID{toID}})
	if e.G.Obj(bladeID).AttachedTo != toID {
		t.Fatalf("attach: blade is on %d, want %d", e.G.Obj(bladeID).AttachedTo, toID)
	}
}

func namedFirst(e *Engine, id state.ObjID) bool {
	return effects.MatchesSpecCtx(e.G, "Card.namedFirst_Name", id, e.specCtx(0, 0))
}

// TestClonedGameSetNameIsScopedToItsOwnEngine is the reviewer's mandated
// regression on the boundary this ticket's first implementation got wrong.
//
// The layer-3 rename that a name filter reads must come from the game the
// filter is evaluating, never from another one. The rejected design put a
// live *rules.Engine back-pointer on state.Game, which state.Game.Clone
// shallow-copied: a cloned game whose SetName applicability then changed still
// answered from the ORIGINAL engine's continuous effects. The replacement
// derives the names in rules and passes them down as an immutable value slice
// on the SpecContext, so the two games cannot see each other by construction.
//
// It asserts both directions: a game cloned BEFORE the rename applies never
// picks it up from the original, and an engine clone whose applicability then
// diverges (the Equipment moves to the other creature) answers from its own
// board while the original stays as it was.
func TestClonedGameSetNameIsScopedToItsOwnEngine(t *testing.T) {
	t.Parallel()
	e, bladeID, bearID, elfID := setNameScopeBoard(t)

	// A state-level clone of the game BEFORE the rename applies. Nothing that
	// happens to the original afterwards may reach it.
	before := e.G.Clone()

	equip(t, e, bladeID, bearID)
	if e.Name(bearID) != "First Name" || !namedFirst(e, bearID) {
		t.Fatalf("original: equipped bear name=%q filter=%v, want the rename",
			e.Name(bearID), namedFirst(e, bearID))
	}
	// THE REGRESSION: the pre-change clone must answer from its own board.
	// The rejected design gave state.Game a live *rules.Engine back-pointer
	// that Game.Clone shallow-copied, so this clone read the ORIGINAL game's
	// continuous effects and reported the rename it never had.
	if effects.MatchesSpecFrom(before, "Card.namedFirst_Name", bearID, 0, 0) {
		t.Fatal("a cloned game must not see a rename that applies only in the original")
	}
	if !effects.MatchesSpecFrom(before, "Card.namedGrizzly_Bears", bearID, 0, 0) {
		t.Fatal("the cloned game's bear must still match its own printed name")
	}

	// The engine-level clone half: a clone owns its rename derivation, so a
	// divergent applicability change is visible on exactly one side.
	c := e.Clone()
	if !namedFirst(c, bearID) {
		t.Fatal("a fresh engine clone must see the rename its own board carries")
	}
	equip(t, c, bladeID, elfID) // the Equipment moves, on the CLONE only
	if namedFirst(c, bearID) {
		t.Fatal("clone: the unequipped bear must no longer match the SetName$ name")
	}
	if !effects.MatchesSpecCtx(c.G, "Card.namedGrizzly_Bears", bearID, c.specCtx(0, 0)) {
		t.Fatal("clone: the unequipped bear must match its printed name again")
	}
	if !namedFirst(c, elfID) {
		t.Fatal("clone: the newly equipped elf must match the SetName$ name")
	}
	if !namedFirst(e, bearID) {
		t.Fatal("original: the bear must still match the SetName$ name after the clone diverged")
	}
	if namedFirst(e, elfID) {
		t.Fatal("original: the elf must not pick up the clone's attachment")
	}
}

// TestBareGameNameFilterReadsThePrintedName pins the documented fallback
// (rules/setname.go): a filter call that rules did not build -- a bare
// *state.Game, a state-level Game.Clone, effects.MatchesSpecFrom from inside a
// resolving effect -- carries no layer-3 names and reads the printed face.
// This is the same reach SpecContext.ExtraTypes has for layer-4 types. It is
// pinned rather than papered over: a future change that gives those call sites
// real names must delete this test deliberately, and a bare game must never
// silently answer from some other game's engine.
func TestBareGameNameFilterReadsThePrintedName(t *testing.T) {
	t.Parallel()
	e, bladeID, bearID, _ := setNameScopeBoard(t)
	equip(t, e, bladeID, bearID)
	if !namedFirst(e, bearID) {
		t.Fatal("precondition: the rules-built context must see the rename")
	}
	for _, tc := range []struct {
		name string
		g    *state.Game
	}{
		{"live game, no rules context", e.G},
		{"state-level clone", e.G.Clone()},
	} {
		if effects.MatchesSpecFrom(tc.g, "Card.namedFirst_Name", bearID, 0, 0) {
			t.Errorf("%s: a context-free filter must not see the layer-3 rename", tc.name)
		}
		if !effects.MatchesSpecFrom(tc.g, "Card.namedGrizzly_Bears", bearID, 0, 0) {
			t.Errorf("%s: a context-free filter must read the printed name", tc.name)
		}
	}
}
