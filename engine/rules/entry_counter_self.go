package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ownEntryCounterExcluded reports whether an AddCounter replacement whose
// source is the very permanent whose entry counters are being settled must
// stay out of that settlement. CR 614.12: a replacement effect that modifies
// how a permanent enters may come from the permanent itself only if it
// affects that permanent alone, not a general subset that includes it. Dr.
// Joseph M'Benga's "a creature you control" doubler therefore does not double
// the +1/+1 counter it enters with (Deanna Troi's rider); a self-scoped line
// (Mowu's "on CARDNAME", recipient spec Card.Self) still applies. recipient
// is the line's ValidCard$/ValidObject$ spec; recv is the object the counters
// are put on (a placement on any other object is not the entrant's entry).
func ownEntryCounterExcluded(settling, source, recv state.ObjID, recipient string) bool {
	if settling == 0 || source != settling || recv != settling {
		return false
	}
	// Card.Self is the corpus's only self-scoped AddCounter recipient (Mowu).
	return !selfScopedSpec(recipient)
}

// entryDestinationAdmits is the Destination$ gate for a Moved replacement.
// It folds in the CR 614.12 own-entry exclusion so an entering Orb of Dreams'
// general "Permanents enter tapped" (ValidCard$ Permanent) does not replace
// its own entry, while a self-scoped line (Card.Self) still does.
func entryDestinationAdmits(g *state.Game, r cards.Repl, source state.ObjID, ev events.Event) bool {
	if d, ok := r.ParamCode(cards.PKDestination); ok && !effects.Destination(d).Admits(ev.To) {
		return false
	}
	return !ownEntryMoveExcluded(g, source, ev, r.ParamStr(cards.PKValidCard))
}

// ownEntryMoveExcluded is the same CR 614.12 rule for a Moved replacement
// on a battlefield entry: a line whose source is the entering object itself
// (not yet on the battlefield) applies to that entry only when its ValidCard$
// is self-scoped. Orb of Dreams' "Permanents enter tapped" (ValidCard$
// Permanent) is a general subset, so the Orb enters untapped; False Floor's
// "CARDNAME enters tapped" (Card.Self) still applies. A line with no
// ValidCard$ (Soulless Jailer's ValidLKI$ Permanent) is not self-scoped.
func ownEntryMoveExcluded(g *state.Game, source state.ObjID, ev events.Event, validCard string) bool {
	if ev.Kind != events.MoveZone || ev.To != state.ZBattlefield || source != ev.Obj {
		return false
	}
	if o := g.Obj(source); o != nil && o.Zone == state.ZBattlefield {
		return false
	}
	return !selfScopedSpec(validCard)
}

// selfScopedSpec reports whether every comma alternative of a ValidCard$
// spec names the source itself (Card.Self, optionally +qualified: Card.Self+
// kicked, Card.Self+!wasCast).
func selfScopedSpec(spec string) bool {
	if strings.TrimSpace(spec) == "" {
		return false
	}
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt != "Card.Self" && !strings.HasPrefix(alt, "Card.Self+") {
			return false
		}
	}
	return true
}

// runEntryBody runs an Updated Moved replacement body that the entry fold did
// not absorb (a body with a SubAbility$ rider, Mikey & Don's "enters with an
// additional +1/+1 counter" Effect) with the entrant marked as settling, so
// the counters it puts on the entrant see the same CR 614.12 self-exclusion
// as a folded grant: Michelangelo, Weirdness to 11's "a creature you control"
// line does not add one to the counter it enters with.
func (e *Engine) runEntryBody(ctx *effects.Ctx, stored *events.Event, with *cards.SA) {
	saved := e.entrySettling
	if stored.Kind == events.MoveZone && stored.To == state.ZBattlefield {
		e.entrySettling = stored.Obj
	}
	e.runReplaceWith(ctx, stored.Obj, with, nil)
	e.entrySettling = saved
}
