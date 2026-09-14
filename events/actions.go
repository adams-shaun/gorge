package events

import "github.com/adams-shaun/gorge/state"

const (
	discardText     = "discarded"
	discardCostText = "discarded as a cost"
)

// Discard returns the canonical zone-change event for a discard effect or
// cleanup discard. Keeping the action marker here means every producer and
// consumer agrees on what distinguishes a discard from an ordinary hand move;
// replacement effects may change To while preserving the marker.
func Discard(obj state.ObjID, player state.PlayerID) Event {
	return Event{Kind: MoveZone, Obj: obj, From: state.ZHand, To: state.ZGraveyard,
		Player: player, Text: discardText}
}

// DiscardCost returns the canonical zone-change event for discarding a card as
// a cost. Cost provenance is distinct so a trigger requiring ValidCause$ does
// not attribute a cost to an unrelated object already on the stack.
func DiscardCost(obj state.ObjID) Event {
	return Event{Kind: MoveZone, Obj: obj, From: state.ZHand, To: state.ZGraveyard,
		Text: discardCostText}
}

// IsDiscard reports whether ev is one of the canonical discard moves. The
// destination is deliberately not part of the test: a replacement can redirect
// the discarded card while the action that caused the move remains a discard.
func IsDiscard(ev Event) bool {
	if ev.Kind != MoveZone || ev.From != state.ZHand {
		return false
	}
	return ev.Text == discardText || ev.Text == discardCostText
}

// IsDiscardCost reports whether ev records the cost form of a discard.
func IsDiscardCost(ev Event) bool {
	return ev.Kind == MoveZone && ev.From == state.ZHand && ev.Text == discardCostText
}
