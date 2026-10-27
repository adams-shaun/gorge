package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// zoneGate implements TriggerZones$: a trigger only fires while its source is
// in one of the listed zones. The default is the battlefield, which is why an
// enchantment's upkeep trigger stops when it is destroyed.
//
// checkTriggers calls this after the event has already been folded into
// state (emit logs before it checks triggers), so o.Zone alone only ever
// reflects the zone the object is in *now*. That is correct for an
// entering-the-zone trigger (Snapcaster's ETB: o.Zone is already
// Battlefield by the time this runs) but wrong for a leaving-the-zone one --
// a plain "dies" trigger (Origin$ Battlefield, Destination$ Graveyard,
// ValidCard$ Card.Self, default TriggerZones$ Battlefield) would never see
// its own source "in" the battlefield, because by the time checkTriggers
// runs the move has already happened and o.Zone reads Graveyard. CR 603.10's
// full "look back in time" is not modeled, but the one case Task 20's own
// ChangesZone mode needs it for is narrow and self-contained: when the event
// under test is itself the zone change of this trigger's own source (source
// == ev.Obj), the zone it was in immediately before (ev.From) counts as well
// as the zone it is in now, so both an ETB and a dies trigger with the
// ordinary default work from the same rule.
func (e *Engine) zoneGate(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	o := e.G.Obj(source)
	if o == nil {
		return false
	}
	if e.effectMatchOverride && source == e.effectMatchSource {
		// An Effect-delivered trigger lives on the Effect object, not on its
		// creating card, so the card's zone is irrelevant (CR 611.2a).
		return true
	}
	spec := t.ParamStr(cards.PKTriggerZones)
	if spec == "" {
		// Forge's ActiveZones$ is the trigger-side spelling of the same gate
		// (the replacement side already reads the key:
		// engine/rules/replacement.go's zone gate). Sower of Discord's two
		// DamageDoneOnce halves declare ActiveZones$ Battlefield; an explicit
		// ActiveZones$ is authoritative exactly like an explicit
		// TriggerZones$, so the two special cases below keep treating it as
		// declared.
		spec = t.ParamStr(cards.PKActiveZones)
	}
	if spec == "" && ev.Kind == events.PutOnStack && source == ev.Obj && t.Mode == "SpellCast" &&
		strings.Contains(t.ParamStr(cards.PKValidCard), "Card.Self") {
		// CR 601.2i: a spell's own cast trigger can observe the spell on the
		// stack, but only when it actually matches that spell. Without the
		// self-card predicate this exception incorrectly admits unrelated
		// triggers carried by a cast object (notably Extort); ordinary zone
		// gating remains the default for those triggers.
		return true
	}
	if spec == "" {
		// "When you discard this card" (Orvar, Bartered Cow, Titanbones: 14
		// of the corpus's Mode$ Discarded lines) declares no TriggerZones$,
		// and the only zone a card is discarded from is its owner's hand (CR
		// 701.9a). Forge applies no zone restriction to a trigger without
		// TriggerZones$; the battlefield default below would leave such a
		// trigger unable to fire at all, so the card's own discard admits it
		// wherever the discard (or a replacement redirecting it) put it.
		if t.Mode == "Discarded" && source == ev.Obj && events.IsDiscard(ev) {
			return true
		}
		// The cycled card itself is the moved card (ValidCard$ Card.Self):
		// its own cycle-trigger must fire from wherever the cost discard (or a
		// replacement redirecting it) put it, the same courtesy the Discarded
		// case above extends.
		if t.Mode == "Cycled" && source == ev.Obj && events.IsDiscard(ev) {
			return true
		}
		// A card's own ChangesZone move from any origin (Lupine Harbingers'
		// hand->exile "note the number of turns you've begun since it was
		// foretold"; Ulamog, the Infinite Gyre and Emrakul, the Aeons Torn
		// "put into a graveyard from anywhere"; Gaea's Blessing and Narcomoeba
		// "from your library"; Golgari Brownscale, graveyard->hand; Geth's
		// Summons, the stack: 19 corpus files at the pin carry a self
		// ChangesZone line with no TriggerZones$ whose destination is not the
		// battlefield and whose origin is neither the battlefield nor the
		// hand, plus Lupine Harbingers). CR 603.6a / 603.10a: the card's own
		// trigger looks back in time and fires from the zone the card left.
		// Forge applies no zone restriction to a trigger without
		// TriggerZones$; the battlefield default would leave such a trigger
		// unable to fire at all, so the card's own move from any origin admits
		// it, the same courtesy the Discarded and Cycled cases above extend.
		// Origin$ and Destination$ stay the trigger matcher's job, so a dies
		// trigger (Origin$ Battlefield) still ignores a mill. A line with
		// neither Origin$ nor Destination$ does not look back in time
		// (zoneChangeNoLookBack) and keeps the post-move zone check below.
		// (Self-moves whose DESTINATION is the battlefield need no
		// admission: the post-move zone check below already sees them.)
		if t.Mode == "ChangesZone" && source == ev.Obj && ev.Kind == events.MoveZone &&
			!zoneChangeNoLookBack(t) {
			return true
		}
		spec = "Battlefield"
	}
	zones := [2]state.Zone{o.Zone, o.Zone}
	n := 1
	if source == ev.Obj && ev.Obj != 0 &&
		(ev.Kind == events.MoveZone || ev.Kind == events.Draw || ev.Kind == events.PutOnStack) &&
		!zoneChangeNoLookBack(t) {
		zones[1] = ev.From
		n = 2
	}
	for _, zone := range zones[:n] {
		if zoneSpecContains(spec, zone) {
			return true
		}
	}
	return false
}

// zoneSpecContains reports whether spec (a Forge TriggerZones value -- a
// comma-separated list of zone names, constant for the life of the card)
// lists want. It scans the string by slicing comma-separated parts apart with
// strings.Cut, which shares the backing string and allocates nothing, instead
// of strings.Split (whose []string is a fresh allocation per call). zoneGate
// runs from the per-event trigger walk -- the same hot path Task A2 fixed the
// zone copy in -- so this avoids churning an allocation for every trigger on
// every event. Parts are handed to effects.ParseZone unchanged (it trims each
// name itself), so the result is byte-identical to the old
// strings.Split+TrimSpace+ParseZone loop, including its handling of empty
// leading/trailing/double-separator segments: an empty zone name parses to
// the graveyard, exactly as it always did.
func zoneSpecContains(spec string, want state.Zone) bool {
	// Walk separator-delimited segments by index so that, like strings.Split,
	// a spec ending in a separator still yields a final empty segment (which
	// effects.ParseZone resolves to the graveyard). strings.Cut would drop
	// that trailing Phantom Graveyard part and change behaviour on a malformed
	// spec; slicing keeps every segment while allocating nothing.
	i := 0
	for {
		j := strings.IndexByte(spec[i:], ',')
		var part string
		if j < 0 {
			part = spec[i:]
		} else {
			part = spec[i : i+j]
		}
		if effects.ParseZone(part) == want {
			return true
		}
		if j < 0 {
			return false
		}
		i += j + 1
	}
}

// zoneChangeNoLookBack reports a ChangesZone line that names neither an
// Origin$ nor a Destination$ (Ghostly Dancers, Bullseye, Death Dealer: "When
// CARDNAME enters", the corpus's only two such lines at the pin). Such a line
// does not look back in time -- Forge's Trigger.looksBackInTime admits a
// ChangesZone line only through its Origin$ or Destination$ -- so its own
// source counts only from the zone it is in AFTER the move: it fires when
// the card enters the battlefield (CR 603.6a) and not when it leaves (the
// CR 603.10a look-back belongs to leaves-the-battlefield abilities, which
// this line is not).
func zoneChangeNoLookBack(t cards.Trigger) bool {
	return t.Mode == "ChangesZone" && !t.HasParam(cards.PKOrigin) && !t.HasParam(cards.PKDestination)
}
