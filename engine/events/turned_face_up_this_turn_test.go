package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTurnedFaceUpThisTurnFold pins the per-object face-up provenance the
// turnedFaceUpThisTurn filter word reads (Kaust, Eyes of the Glade's
// DamageDone trigger): the TurnFaceUp fold sets it on a REAL face-down
// battlefield turn-up and on nothing else, and the fact ends at the turn
// boundary and with the object's zone (CR 400.7) -- the same two clears
// ActivatedThisTurn carries.
func TestTurnedFaceUpThisTurnFold(t *testing.T) {
	mk := func(faceDown bool) (*state.Game, state.ObjID) {
		g, id := gameWithOneCard(t)
		o := g.Obj(id)
		g.SetZone(state.ZHand, 0, nil)
		g.SetZone(state.ZBattlefield, 0, []state.ObjID{id})
		o.Zone = state.ZBattlefield
		o.FaceDown = faceDown
		if o.TurnedFaceUpThisTurn {
			t.Fatalf("precondition: a fresh battlefield object carries the mark")
		}
		return g, id
	}

	g, id := mk(true)
	Apply(g, Event{Kind: TurnFaceUp, Obj: id})
	o := g.Obj(id)
	if o.FaceDown || !o.TurnedFaceUpThisTurn {
		t.Fatalf("face-down battlefield turn-up: faceDown=%v mark=%v, want false/true", o.FaceDown, o.TurnedFaceUpThisTurn)
	}

	// The fold's face-down gate: an already face-up permanent emits the
	// event but gains no provenance (and a non-battlefield object never
	// turns up at all).
	g2, id2 := mk(false)
	Apply(g2, Event{Kind: TurnFaceUp, Obj: id2})
	if g2.Obj(id2).TurnedFaceUpThisTurn {
		t.Fatalf("face-up permanent gained the mark")
	}
	g3, id3 := gameWithOneCard(t) // still in hand
	Apply(g3, Event{Kind: TurnFaceUp, Obj: id3})
	if g3.Obj(id3).TurnedFaceUpThisTurn {
		t.Fatalf("hand card gained the mark")
	}

	// The turn boundary ends the fact, like every other this-turn flag.
	Apply(g, Event{Kind: TurnChange, Player: 1, Amount: 2})
	if o.TurnedFaceUpThisTurn {
		t.Fatalf("TurnChange left the mark set")
	}

	// CR 400.7: a returning permanent is a new object and never carries the
	// old object's face-up provenance.
	g4, id4 := mk(true)
	o4 := g4.Obj(id4)
	Apply(g4, Event{Kind: TurnFaceUp, Obj: id4})
	if !o4.TurnedFaceUpThisTurn {
		t.Fatalf("precondition: turn-up did not stamp")
	}
	g4.SetZone(state.ZBattlefield, 0, nil)
	g4.SetZone(state.ZGraveyard, 0, []state.ObjID{id4})
	Apply(g4, Event{Kind: MoveZone, Obj: id4, From: state.ZBattlefield, To: state.ZGraveyard})
	if o4.TurnedFaceUpThisTurn {
		t.Fatalf("MoveZone left the mark set: a new object must not carry it")
	}
}
