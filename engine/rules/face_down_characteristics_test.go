package rules

// CR 708.2a face-down characteristics in the filter grammar
// (cr-g13-face-characteristics-mvc). A face-down permanent is a 2/2 creature
// with no text, no name, no subtypes and no mana cost, so it has mana value 0
// and no colour, and it has the same name as nothing (CR 201.2a). The layer
// walk already hides its printed face (Derived Colors and Name are empty);
// these tests pin the filter reads targeting and matching use. wantCmc is
// face_mana_value_test.go's.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// faceDownOnBoard puts c onto the battlefield face down for seat 0 through
// the logged face-down entry (events.FaceDownEntryCounter).
func faceDownOnBoard(t *testing.T, e *Engine, c *cards.Card) state.ObjID {
	t.Helper()
	id := handCard(e, c, 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield,
		Counter: events.FaceDownEntryCounter, Secret: true})
	if o := e.G.Obj(id); !o.FaceDown || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: %d face down=%v zone=%s", id, o.FaceDown, o.Zone)
	}
	return id
}

// TestManaValueFaceDownPermanentIsZero: CR 708.2a. A face-down Exalted Angel
// ({4}{W}{W}) has no mana cost while face down.
func TestManaValueFaceDownPermanentIsZero(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	angel := faceDownOnBoard(t, e, mustCorpusCard(t, reg, "Exalted Angel"))
	wantCmc(t, e, angel, 0, "face-down Exalted Angel (CR 708.2a)")
}

// TestFaceDownPermanentIsColourless: CR 708.2a. A face-down Grinning Demon
// ({2}{B}{B}) is colourless, so Doom Blade's "Creature.nonBlack" may target
// it and Ultimate Price's "Creature.MonoColor" may not.
func TestFaceDownPermanentIsColourless(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	demon := faceDownOnBoard(t, e, mustCorpusCard(t, reg, "Grinning Demon"))
	sc := e.specCtx(0, 0)
	if !e.matchesSpec("Creature.nonBlack", demon, sc) {
		t.Error("face-down Grinning Demon does not match Doom Blade's Creature.nonBlack (read its printed black)")
	}
	if e.matchesSpec("Creature.MonoColor", demon, sc) {
		t.Error("face-down Grinning Demon matches Ultimate Price's Creature.MonoColor (read its printed black)")
	}
	if e.matchesSpec("Creature.Black", demon, sc) {
		t.Error("face-down Grinning Demon matches Creature.Black")
	}
}

// TestFaceDownPermanentHasNoName: CR 708.2a/201.2a. A face-down Llanowar
// Elves has no name, so "named Llanowar Elves" does not match it.
func TestFaceDownPermanentHasNoName(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	elves := faceDownOnBoard(t, e, mustCorpusCard(t, reg, "Llanowar Elves"))
	sc := e.specCtx(0, 0)
	if e.matchesSpec("Card.namedLlanowar Elves", elves, sc) {
		t.Error("face-down Llanowar Elves matches namedLlanowar Elves (read its printed name)")
	}
	if !e.matchesSpec("Creature.noName", elves, sc) {
		t.Error("face-down Llanowar Elves does not match noName")
	}
}

// TestFaceUpAgainRestoresPrintedCharacteristics guards the zone and state
// gate: once the Demon is turned face up it is black, mana value 4 and named
// again.
func TestFaceUpAgainRestoresPrintedCharacteristics(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	demon := faceDownOnBoard(t, e, mustCorpusCard(t, reg, "Grinning Demon"))
	e.emit(events.Event{Kind: events.TurnFaceUp, Obj: demon})
	sc := e.specCtx(0, 0)
	if !e.matchesSpec("Creature.Black", demon, sc) || !e.matchesSpec("Card.namedGrinning Demon", demon, sc) {
		t.Error("face-up Grinning Demon lost its printed colour or name")
	}
	wantCmc(t, e, demon, 4, "face-up Grinning Demon")
}
