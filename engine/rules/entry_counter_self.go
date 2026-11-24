package rules

import (
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

// ownEntryMoveExcluded is the same CR 614.12 rule for a printed Moved
// replacement on a battlefield entry: a line whose source is the entering
// object itself (not yet on the battlefield) applies to that entry only when
// its ValidCard$ is self-scoped. Orb of Dreams' "Permanents enter tapped"
// (ValidCard$ Permanent) and Kira Nerys, Firebrand's "a creature you control"
// are general subsets, so neither touches its own entry; False Floor's
// "CARDNAME enters tapped" (Card.Self) still applies. A line with no
// ValidCard$ (Soulless Jailer's ValidLKI$ Permanent) is not self-scoped. Only
// printed R: lines reach it (replacementMatchesRemembered): an Effect-created
// line whose source is the entrant (Storied Veteran's finality rider,
// Worldheart Phoenix's counters, ValidCard$ Card.IsRemembered or
// Card.EffectSource) comes from that Effect, not from the permanent.
func ownEntryMoveExcluded(g *state.Game, r cards.Repl, source state.ObjID, ev events.Event) bool {
	if r.EventKind() != cards.ReplMoved || ev.Kind != events.MoveZone || ev.To != state.ZBattlefield || source != ev.Obj {
		return false
	}
	if o := g.Obj(source); o != nil && o.Zone == state.ZBattlefield {
		return false
	}
	return !selfScopedSpec(r.ParamStr(cards.PKValidCard))
}

// selfScopedSpec reports whether every comma alternative of a ValidCard$
// spec names the source itself (Card.Self, optionally +qualified: Card.Self+
// kicked, Card.Self+!wasCast).
func selfScopedSpec(spec string) bool {
	const self = "Card.Self"
	start := 0
	for i := 0; i <= len(spec); i++ {
		if i < len(spec) && spec[i] != ',' {
			continue
		}
		alt := spec[start:i]
		for len(alt) > 0 && alt[0] == ' ' {
			alt = alt[1:]
		}
		for len(alt) > 0 && alt[len(alt)-1] == ' ' {
			alt = alt[:len(alt)-1]
		}
		if alt != self && (len(alt) <= len(self) || alt[:len(self)+1] != self+"+") {
			return false
		}
		start = i + 1
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
