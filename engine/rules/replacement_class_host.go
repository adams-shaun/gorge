package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Narrow interfaces for the replacement classes' free functions. The
// engineSurface census (internal/codeshape) is shrink-only: a new class arm
// may not add an *Engine method, a free function taking *Engine, or a
// method on a struct holding one, so each arm hangs off the face of
// *Engine it actually needs and call sites pass the engine in. *Engine
// satisfies every one of these by its existing methods.

// replClassHost is the face a class matcher arm needs: the game for
// player-spec matching and the object-spec matcher. The shared condition
// gate is passed as the closure the call site builds over the line's own
// r/source/you/svars frame, so the arm needs no map-typed parameter.
type replClassHost interface {
	Game() *state.Game
	matchesSpecFrom(spec string, id state.ObjID, you state.PlayerID, source state.ObjID) bool
}

// replCountPricer is the face the proliferate count rewrite needs: the ONE
// shared ReplaceCount$ arithmetic.
type replCountPricer interface {
	replaceCount(source state.ObjID, r *cards.Repl, name string, amount int32) (int32, bool)
}

// replNoteEmitter is the face the CantHappen suppression needs: the event
// emitter the prevented-Note convention goes through.
type replNoteEmitter interface {
	emit(ev events.Event) events.Event
}

// damageClassHost is the face the repl:DamageDone dispatch arm needs.
type damageClassHost interface {
	Game() *state.Game
	applicableDamageReplacements(ev events.Event, matches []replMatch) []replMatch
	damageAffectedPlayer(ev events.Event) (state.PlayerID, bool)
	poseDamageReplacementChoice(ev events.Event, matches []replMatch, p state.PlayerID) (events.Event, bool)
	replacementAskPlayer(matches []replMatch, affected state.PlayerID) state.PlayerID
	applyNonMoveReplacements(ev events.Event, matches []replMatch) (events.Event, bool)
}
