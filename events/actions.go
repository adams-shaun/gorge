package events

import "github.com/adams-shaun/gorge/state"

const (
	discardText     = "discarded"
	discardCostText = "discarded as a cost"
)

// Discard returns the canonical zone-change event for a discard effect or
// cleanup discard. Keeping the action marker here means every producer and
// consumer agrees on what distinguishes a discard from an ordinary hand move;
// replacement effects may change To while preserving the marker. Mode$ Random
// additionally carries its one-based eligible-hand choice index in Amount;
// ordinary discards leave Amount zero. Apply folds the chosen Obj's move, so
// the random choice and its state change share a single replayable event.
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

const sacrificeText = "sacrificed"

// Sacrifice returns the canonical zone-change event for sacrificing a
// permanent (CR 701.21a), whether as a cost or by an effect. Every producer
// uses it so the Sacrificed trigger and a replacement redirecting the move
// agree on the marker.
func Sacrifice(obj state.ObjID) Event {
	return Event{Kind: MoveZone, Obj: obj, From: state.ZBattlefield, To: state.ZGraveyard, Text: sacrificeText}
}

// IsSacrifice reports whether ev is a sacrifice: a permanent moved from the
// battlefield under the "sacrificed" action marker (CR 701.21a). Like
// IsDiscard it ignores the destination, which a replacement may change.
func IsSacrifice(ev Event) bool {
	return ev.Kind == MoveZone && ev.From == state.ZBattlefield && ev.Text == sacrificeText
}

// ActionMarker returns the action marker a zone change carries when it is a
// sacrifice or a discard, and "" for every other event. A replacement that
// substitutes a different move for such an event records this marker so the
// substituted move can carry it (see CarryAction).
func ActionMarker(ev Event) string {
	if IsSacrifice(ev) || IsDiscard(ev) {
		return ev.Text
	}
	return ""
}

// CarryAction re-labels the move a replacement substituted for an action
// event. CR 614.6: the replaced event never happens, but the modified event
// is still the same action -- a permanent sacrificed "into exile instead" was
// sacrificed (CR 701.21a), and a card discarded "onto the battlefield instead"
// was discarded (CR 701.9a). Only the replaced object's first move out of the
// zone the action moved it from is re-labelled, and only when that move has
// no marker of its own, so a later move of the same object by the same
// replacement body stays an ordinary zone change.
func CarryAction(marker string, obj state.ObjID, ev Event) Event {
	if marker == "" || obj == 0 || ev.Kind != MoveZone || ev.Obj != obj || ev.Text != "" {
		return ev
	}
	var from state.Zone
	switch marker {
	case sacrificeText:
		from = state.ZBattlefield
	case discardText, discardCostText:
		from = state.ZHand
	default:
		return ev
	}
	if ev.From != from {
		return ev
	}
	ev.Text = marker
	return ev
}
