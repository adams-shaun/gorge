package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// exileCounterRider reports whether the dies-to-exile body w is the plain
// exile move of ReplacedCard that also leaves counters on the exiled card:
// WithCountersType$ on the move itself (Dauthi Voidwalker's VOID, Draugr
// Necromancer's ICE, Rayami's BLOOD), or RememberChanged$ True plus a
// PutCounter sub-ability on the Remembered card (Ravenloft Adventurer's HIT).
// A WithCountersAmount$ with no kind is not a counter rider, and a bare
// PutCounter on something else is not the rider chain. The bodies this
// predicate still catches (the sub-ability chain) have no lowering; they are
// refused by name rather than by the generic "Moved to Graveyard shape".
func exileCounterRider(w *cards.SA) bool {
	if w == nil || w.API != "ChangeZone" {
		return false
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" {
		return false
	}
	switch bp.str("Origin") {
	case "", "All", "Battlefield":
	default:
		return false
	}
	if _, ok := bp.get("WithCountersType"); ok {
		return true
	}
	if _, ok := bp.get("WithCountersAmount"); ok {
		return false // an amount with no kind is not a counter rider
	}
	if w.Sub == nil || w.Sub.API != "PutCounter" || w.Sub.Params["Defined"] != "Remembered" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(bp.str("RememberChanged")), "True")
}
