package view

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestLibraryOrderRedaction is the Ruling J1 redaction leaf: a LibraryOrder
// event with Secret: true must be redacted from a non-owner's view (the
// exact order of a hidden zone is private to its owner), while the owner's
// own projection keeps the payload. Asserted against the OTHER seat, not the
// owner, because the owner seeing their own secret is the case a naive
// redaction test gets backwards.
//
// The Omniscient arm is the one this leaf watches: view.RedactEventFor
// redacts a Secret event revealing library order -- and LibraryOrder was
// added to that disjunction explicitly (Ruling J1) so the chosen new order
// does not leak to an omniscient spectator the way a Shuffle already does
// not.
//
// The event is deliberately given a NON-library To (state.ZHand) so this
// leaf exercises the LibraryOrder term of the disjunction rather than the
// pre-existing "To == ZLibrary" clause, which would otherwise redact a
// To==0 LibraryOrder all by itself and make removing the LibraryOrder term
// undetectable. Removing LibraryOrder from the disjunction must fail the
// Omniscient assertion below.
func TestLibraryOrderRedaction(t *testing.T) {
	// Two seats so viewer 1 is genuinely a different seat from Player 0. The
	// IDs need not resolve to real objects: a Secret event for a non-owner is
	// redacted by shape alone, and the Omniscient arm matches on Kind + Secret
	// without reading the game.
	g := state.NewGame([]string{"a", "b"})
	ev := events.Event{Seq: 7, Kind: events.LibraryOrder, Player: 0,
		To: state.ZHand, IDs: []state.ObjID{1, 2, 3}, Secret: true}

	// The owner's OWN seat projection keeps the payload: their secret.
	owner := RedactEvent(g, ev, 0)
	if len(owner.IDs) != 3 {
		t.Fatalf("owner's own LibraryOrder redacted: got IDs %v, want the full order", owner.IDs)
	}

	// The OTHER seat's projection strips the payload to the event's shape.
	other := RedactEvent(g, ev, 1)
	if len(other.IDs) != 0 {
		t.Fatalf("non-owner's LibraryOrder leaked its IDs %v (must be redacted to shape-only)", other.IDs)
	}
	if other.Kind != events.LibraryOrder || other.Player != 0 || !other.Secret {
		t.Fatalf("redacted event lost its shape: %+v", other)
	}

	// An omniscient spectator never sees library order, owner or not.
	omni := RedactEventFor(g, ev, 1, Omniscient)
	if len(omni.IDs) != 0 || omni.Obj != 0 {
		t.Fatalf("omniscient LibraryOrder leaked its payload %+v (must be redacted to shape-only)", omni)
	}
	if omni.Kind != events.LibraryOrder || omni.Player != 0 || !omni.Secret {
		t.Fatalf("omniscient redaction lost the event's shape: %+v", omni)
	}
}
