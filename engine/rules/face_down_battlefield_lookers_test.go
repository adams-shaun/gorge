package rules

// CR 708.5 -- only a face-down permanent's controller may look at it unless
// an effect allows another player to. Found Footage's printed static ("You
// may look at face-down creatures your opponents control any time",
// MayLookAt$ You over Creature.faceDown+OppCtrl, AffectedZone$ Battlefield)
// and Lumbering Laundry's activated effect (StaticAbilities$ MayLookFaceDown
// over Creature.faceDown+YouDontCtrl, until end of turn) are exactly such
// effects: the board shows the face to the seats they name, and to nobody
// else.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// faceDownOnBattlefield moves name from p's hand/library onto p's battlefield
// face down through the manifest marker, and returns the id.
func faceDownOnBattlefield(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != name {
				continue
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield, Counter: events.ManifestEntryCounter})
			return id
		}
	}
	t.Fatalf("card %q not found in seat %d's hand/library", name, p)
	return 0
}

// battlefieldFaceIn is the Name seat viewer's projection shows for the
// face-down battlefield permanent id (controlled by ctrl): "" when hidden.
func battlefieldFaceIn(t *testing.T, e *Engine, viewer, ctrl state.PlayerID, id state.ObjID) string {
	t.Helper()
	v := view.ProjectFor(e.G, e, viewer, view.Seat, nil)
	for _, pv := range v.Players {
		if pv.ID != ctrl {
			continue
		}
		for _, cv := range pv.Battlefield {
			if cv.ID != id {
				continue
			}
			if !cv.FaceDown {
				t.Fatalf("precondition: permanent %d is projected face up to seat %d", id, viewer)
			}
			return cv.Name
		}
	}
	t.Fatalf("permanent %d is not on seat %d's battlefield in seat %d's projection", id, ctrl, viewer)
	return ""
}

// TestFoundFootageShowsOpponentsFaceDownCreature: seat 0 controls Found
// Footage; seat 1 controls a face-down 2/2 (a manifested Grizzly Bears). CR
// 708.5's effect exception shows seat 0 the face; seat 1, its controller,
// sees it anyway; a seat 0 with no look grant in play sees nothing.
func TestFoundFootageShowsOpponentsFaceDownCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ff := lookup(t, reg, "Found Footage")
	bears := lookup(t, reg, "Grizzly Bears")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{ff}, []*cards.Card{bears})
	moveByName(t, e, 0, "Found Footage", state.ZBattlefield)
	bear := faceDownOnBattlefield(t, e, 1, "Grizzly Bears")
	if got := battlefieldFaceIn(t, e, 1, 1, bear); got != "Grizzly Bears" {
		t.Errorf("board: the face-down creature's controller is shown face %q, want Grizzly Bears", got)
	}
	if got := battlefieldFaceIn(t, e, 0, 1, bear); got != "Grizzly Bears" {
		t.Errorf("board: Found Footage's controller is shown face %q, want Grizzly Bears", got)
	}
	// The control: the same board without the grant -- seat 0 stays blind.
	e2, _ := corpusEngineCfg(t, reg, nil, []*cards.Card{bears})
	bear2 := faceDownOnBattlefield(t, e2, 1, "Grizzly Bears")
	if got := battlefieldFaceIn(t, e2, 0, 1, bear2); got != "" {
		t.Errorf("board: seat 0 is shown face %q with no look grant in play, want hidden", got)
	}
}

// TestFoundFootageDoesNotShowOwnHiddenToOpponent: Found Footage names
// Creature.faceDown+OppCtrl -- the face-down creatures seat 0's OPPONENTS
// control. Seat 0's own face-down creature is matched by nothing, so seat 1's
// projection of it stays hidden (CR 708.5).
func TestFoundFootageDoesNotShowOwnHiddenToOpponent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ff := lookup(t, reg, "Found Footage")
	bears := lookup(t, reg, "Grizzly Bears")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{ff, bears}, []*cards.Card{bears})
	moveByName(t, e, 0, "Found Footage", state.ZBattlefield)
	bear := faceDownOnBattlefield(t, e, 0, "Grizzly Bears")
	if got := battlefieldFaceIn(t, e, 0, 0, bear); got != "Grizzly Bears" {
		t.Errorf("board: the controller is shown face %q, want Grizzly Bears", got)
	}
	if got := battlefieldFaceIn(t, e, 1, 0, bear); got != "" {
		t.Errorf("board: the opponent is shown face %q of seat 0's own face-down creature, want hidden", got)
	}
}

// TestLumberingLaundryLookLastsThisTurn: seat 0 activates Lumbering Laundry's
// {2} ability. The MayLookFaceDown static registers a look grant (no
// "unimplemented" Note), seat 0 sees seat 1's face-down creature's face for
// the rest of the turn, and after the turn passes it is hidden again.
func TestLumberingLaundryLookLastsThisTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	laundry := lookup(t, reg, "Lumbering Laundry")
	bears := lookup(t, reg, "Grizzly Bears")
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{laundry}, []*cards.Card{bears})
	src := moveByName(t, e, 0, "Lumbering Laundry", state.ZBattlefield)
	bear := faceDownOnBattlefield(t, e, 1, "Grizzly Bears")
	addMana(t, e, 0, "CC")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("seat 0 does not hold priority: %+v", d)
	}
	submitChoices(t, e, abilityOptionFor(t, e, src).Index)
	passUntilStackEmpty(t, e, 20)
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			t.Errorf("stream: activating Lumbering Laundry logged %q", ev.Text)
		}
	}
	if got := battlefieldFaceIn(t, e, 0, 1, bear); got != "Grizzly Bears" {
		t.Errorf("board: the activated look grant shows seat 0 face %q, want Grizzly Bears", got)
	}
	// CR 514.2: the until-end-of-turn grant expires with the turn.
	driveToStepAll(t, e, e.G.Turn+1, 1-e.G.Active, state.StepMain1)
	if got := battlefieldFaceIn(t, e, 0, 1, bear); got != "" {
		t.Errorf("board: after the turn passed seat 0 is still shown face %q, want hidden", got)
	}
}
