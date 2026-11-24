package effects

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func init() {
	Register("Airbend", effAirbend)
}

// AirbendExileCounter is the MoveZone Counter marker an api:Airbend resolution
// stamps on every exile move it performs (CR 701.65a). engine/rules/airbend.go reads
// it off the log to decide when the exiled card's owner may cast it for {2}
// rather than its mana cost: the permission lasts exactly as long as the
// marker-carrying move is the card's most recent move into exile. The value is
// inert at the events.Apply fold (an unknown exile Counter falls to the
// default branch, which only honours the exiled-with IDs payload), so it is a
// pure provenance marker.
const AirbendExileCounter = "airbend"

// effAirbend is Forge's AirbendEffect, CR 701.65a: "Airbend [a permanent]"
// means exile it, and "for as long as it remains exiled, its owner may cast it
// by paying {2} rather than its mana cost."
//
// The exile half lives here; the recast half lives in rules (engine/rules/airbend.go's
// log-scan predicate + engine/rules/legal.go's exile-walk offer + engine/rules/cast.go's
// airbend_cast cost), because a play permission is consumed by the offer and
// beginCast machinery, none of which effects may reach.
//
// The targets arrive through the ordinary Defined() read, so every corpus shape
// is one walk: an SP$/DB$ Airbend with ValidTgts$ names the interactively
// chosen targets (Airbending Lesson's "Airbend target nonland permanent",
// Whirlwind Technique's "up to two target creatures" -- TargetMin$ 0 simply
// resolves to an empty set here, which is the "up to" contract), and a Defined$
// body names its own list (Avatar's Wrath's "airbend all other creatures",
// Monk Gyatso's Defined$ TriggeredTarget). Each target is exiled with the
// shared moveZoneEvent constructor so the MoveZone carries the same
// CantExile-guarded, replay-visible shape every other exile mover emits -- the
// AirbendExileCounter marker on top is this primitive's only addition.
func effAirbend(h Host, c *Ctx, sa *cards.SA) {
	bent := false
	// "Airbend all other creatures" exiles the whole set at once (CR 701.65a
	// airbends each object in the one instruction; CR 603.10a: a
	// leaves-the-battlefield trigger looks back at the board before ANY of
	// them left). The departures are parked as one batch so Suki, Courageous
	// Rescuer, airbent with the others, still sees them leave.
	var bender []state.ObjID
	for _, t := range Defined(h, c, sa) {
		if o := h.Game().Obj(t.Obj); !t.IsPlayer && o != nil && o.Zone == state.ZBattlefield {
			bender = append(bender, o.ID)
		}
	}
	if len(bender) > 0 {
		h.BatchDepartures(bender)
		defer h.EndBatchDepartures()
	}
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil {
			continue
		}
		// A CantExile restriction (The Master, Multiplied) withholds the
		// object entirely: it never leaves its zone, no MoveZone is emitted
		// and the recast permission never attaches -- the same guard the
		// shared ChangeZone settle path applies to every other exile mover.
		if h.ExileBlocked(o.ID, false) {
			continue
		}
		ev := moveZoneEvent(c, o.ID, o.Zone, state.ZExile)
		ev.Counter = AirbendExileCounter
		h.Emit(ev)
		bent = true
	}
	if bent {
		h.Emit(events.Event{Kind: events.ElementalBend, Obj: c.Source, Player: c.Controller, Text: "air"})
	}
}
