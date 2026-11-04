package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// EntryStateBody reports whether a Moved replacement's body is the
// "enters tapped and/or attacking" shape (CR 614.12, 508.4): a ChangeZone to
// the battlefield of the ReplacedCard from anywhere, whose only content is the
// Tapped$ and Attacking$ entry riders (Kira Nerys, Firebrand). Run as the
// ordinary body it moves the replaced card there with those riders; a token
// minted straight onto the battlefield has no move to redirect, so
// ApplyEntryStateBody applies the same riders to it in place.
func EntryStateBody(sa *cards.SA) bool {
	if sa == nil || sa.APIKind() != cards.APIChangeZone || sa.Sub != nil {
		return false
	}
	cz := ChangeZoneOf(sa)
	return cz.DestinationKnown && cz.Destination == state.ZBattlefield &&
		DefinedRefOf(sa).Text == "ReplacedCard" &&
		(cz.OriginAll || !cz.OriginPresent) &&
		(cz.Tapped || strings.EqualFold(cz.Riders.Attacking, "True"))
}

// ApplyEntryStateBody applies an EntryStateBody's riders to id, a permanent
// that has already entered the battlefield without a move (a minted token):
// the entry Tap when Tapped$ is set, then the CR 508.4 attacking entry, whose
// defender the entering permanent's controller chooses. No attack is
// declared, so nothing triggers on attacking (CR 508.4).
func ApplyEntryStateBody(h Host, c *Ctx, sa *cards.SA, id state.ObjID) {
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		return
	}
	cz := ChangeZoneOf(sa)
	if cz.Tapped {
		h.Emit(events.Event{Kind: events.Tap, Obj: id, Player: o.Controller, Text: "entered tapped"})
	}
	rider := classifyAttackingEntryText(h, c, cz.Riders.Attacking, state.ZBattlefield, o.Controller)
	rider.apply(h, c, id, o.Controller, state.ZBattlefield)
}
