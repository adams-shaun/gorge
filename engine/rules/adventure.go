package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// adventureZoneMoveMarker is the MoveZone Counter value that marks the exile
// of a RESOLVED Adventure spell (CR 715.3d). moveResolvedOffStack emits it on
// the FlagAdventure branch, and adventureZoneAvailable requires it: a counter
// -- including one whose graveyard move a replacement redirects to exile --
// never carries it, so the play permission cannot be inferred from the
// absence of a counter label.
const adventureZoneMoveMarker = "adventure_zone"

// adventure.go implements the Adventure spell face (CR 714, Forge's
// AlternateMode:Adventure -- 151 corpus files: a permanent front and an
// Instant/Sorcery "Adventure" back). Two offers ride machinery that already
// exists for other shapes:
//
//   - from the hand, the Adventure spell face is offered as Mode
//     "adventure_alt" (the Room alternate-cast shape, engine/rules/legal.go's hand
//     walk), gated on the ADVENTURE face's own timing, targets and cost;
//     beginCast flips the card to the spell face, and the resolution exiles
//     it into the adventure zone via spellRestZone's FlagAdventure branch,
//     where it is its main face again (CR 715.4: events.Apply's MoveZone
//     fold resets an Adventure card's face on every stack exit).
//   - from exile -- the adventure zone, where the resolving Adventure spell
//     rests -- the main face is offered as Mode "adventure_recast" (the Warp
//     recast shape, engine/rules/legal.go's exile walk), gated on a log-derived
//     adventure-zone provenance check.
//
// The provenance check is log-derived because CastFlags reset on every
// stack->non-battlefield move (engine/events/apply.go), which is exactly the move a
// resolving Adventure spell makes -- no object flag can carry "this card
// sits in the adventure zone". The walk mirrors warpRecastAvailable's
// fixed-order backwards scan without warp's later-turn requirement: CR
// 715.3d permits playing the card from exile immediately.

// isAdventureSpellFace reports whether a face is an Adventure spell half:
// an Instant or Sorcery carrying the Adventure type.
func isAdventureSpellFace(f *cards.Face) bool {
	if f == nil || (!f.IsInstant() && !f.IsSorcery()) {
		return false
	}
	for _, t := range f.Types {
		if t == "Adventure" {
			return true
		}
	}
	return false
}

// adventureSpellFace returns the Adventure spell face (face 1 -- ALTERNATE
// starts face 1 in engine/cards/parse.go) of a card whose AlternateMode is
// Adventure, or nil when the object is not a well-formed Adventure card.
func adventureSpellFace(o *state.Object) *cards.Face {
	if o == nil || o.Card == nil || o.Card.AlternateMode != "Adventure" || len(o.Card.Faces) != 2 {
		return nil
	}
	if !isAdventureSpellFace(o.Card.Faces[1]) {
		return nil
	}
	return o.Card.Faces[1]
}

// isOmenSpellFace reports whether a face is an Omen spell half: an Instant or
// Sorcery carrying the Omen type (CR 720.2).
func isOmenSpellFace(f *cards.Face) bool {
	if f == nil || (!f.IsInstant() && !f.IsSorcery()) {
		return false
	}
	for _, t := range f.Types {
		if t == "Omen" {
			return true
		}
	}
	return false
}

// omenSpellFace returns the Omen spell face (face 1 -- ALTERNATE starts face
// 1 in engine/cards/parse.go) of a card whose AlternateMode is Omen, or nil
// when the object is not a well-formed Omen card. CR 720.4: the card has only
// its normal characteristics in every zone except the stack, so this face is
// reachable only while the card is being cast as an Omen.
func omenSpellFace(o *state.Object) *cards.Face {
	if o == nil || o.Card == nil || o.Card.AlternateMode != "Omen" || len(o.Card.Faces) != 2 {
		return nil
	}
	if !isOmenSpellFace(o.Card.Faces[1]) {
		return nil
	}
	return o.Card.Faces[1]
}

// adventureZoneAvailable reports whether id -- which the caller has
// established is in exile (as its main face, CR 715.4) -- got there by
// RESOLVING an adventure_alt cast. That is the only adventure-zone entry this
// engine implements (CR 714.3b's exiled-by-some-other-effect shape is out of
// scope), so the gate is the provenance itself, derived from the log exactly
// like warpRecastAvailable: walking backwards from the log's end,
//
//  1. the most recent MoveZone taking id to exile, which must be the
//     resolved-Adventure move itself -- it carries adventureZoneMoveMarker,
//     emitted by moveResolvedOffStack's FlagAdventure branch. CR 715.3d
//     exiles the card "as it resolves", so a spell countered to exile
//     (Dissipate) never earns the play permission, and neither does a
//     countered card whose graveyard move a replacement redirects to exile
//     (Rest in Peace, Leyline of the Void): the replacement's own move
//     carries no marker,
//  2. scanning back before it, the FIRST event of id encountered must be the
//     adventure-flagged CastInfo -- any MoveZone of id in between (the card
//     left exile and returned by some other route: a recast that resolved to
//     the battlefield, a death, a later exile) means the exile did not come
//     from an Adventure resolution, and an unflagged CastInfo means a later
//     cast reset the provenance.
//
// Everything is a fixed-order walk of the log, so a replayed game derives the
// same answer.
func (e *Engine) adventureZoneAvailable(id state.ObjID) bool {
	log := e.L.Events
	exileIdx := -1
	for i := len(log) - 1; i >= 0; i-- {
		if ev := log[i]; ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZExile {
			exileIdx = i
			break
		}
	}
	if exileIdx < 0 {
		return false
	}
	// The positive marker moveResolvedOffStack stamps on the resolved
	// Adventure's stack->exile move is the only provenance accepted. Every
	// counter path (engine/effects/resolve.go, target_ask.go,
	// resolution_modes.go, misc.go's ward) emits an unmarked move, and a
	// replacement that redirects a countered card's graveyard move to exile
	// (Rest in Peace, Leyline of the Void) discards the original event and
	// logs its own unmarked move, so neither can pass this check.
	if ev := log[exileIdx]; ev.From != state.ZStack || ev.Counter != adventureZoneMoveMarker {
		return false
	}
	for i := exileIdx - 1; i >= 0; i-- {
		ev := log[i]
		if ev.Kind == events.MoveZone && ev.Obj == id {
			return false
		}
		if ev.Kind == events.CastInfo && ev.Obj == id {
			return events.FlagsFrom(ev.Counter)&state.FlagAdventure != 0
		}
	}
	return false
}
