package bots

import (
	"context"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
)

// MatchContext is the per-match information a seat needs before its first
// decision and cannot recover from a view alone. The host attaches it once,
// after the live engine exists, through MatchSeat.AttachMatch. A seat that
// does not implement MatchSeat is unaffected.
//
// Game is the game the seat will answer for, on whichever engine runs it. A
// host always sets it; a bench or command driving the rules engine itself may
// leave it nil. Engine is that game's rules engine, nil on any other engine; a
// seat that needs it refuses the match from AttachMatch. Both must be read,
// never written: the host owns every mutation through events.Apply. A host undo
// replaces both on the match goroutine under the Projection lock, before any
// seat's MatchRewound, so read them there or under Projection.
type MatchContext struct {
	Context context.Context
	Game    game.Game
	Engine  *rules.Engine
	// Projection serializes a seat's engine reads against the host's view
	// projections when the host supplies one. The host projects subscriber
	// views under its match lock; a seat that reads rules.Derived (identity
	// refs, mapping, a heuristic fallback) shares the engine's scratch walk
	// and must run those reads under the same lock. It must never wrap a
	// blocking agent round-trip. Nil means the caller has no concurrent
	// projector and the read runs directly.
	Projection func(f func())
	// Registry is the card corpus when the caller has one; the host passes its
	// BotDeps.Cards. Bench callers may leave it nil.
	Registry *cards.Registry

	GameID string
	Seat   int
	Seats  int
	Format string
	// Seed is the caller's deterministic match seed material. A seat derives
	// its own agent seed from it; it is not a protocol secret.
	Seed []byte
	// Secret keys the v2 object ids. It is derived from Seed when the host
	// does not supply one.
	Secret []byte

	OwnDeck      *deck.Manifest
	OpponentDeck *deck.Manifest
	// OpponentVisible is false when the table does not reveal decklists to
	// this seat; OpponentDeck is then nil.
	OpponentVisible bool

	Mulligan     string
	StartingSeat int
	DeadlineMS   int
	Logf         func(format string, args ...any)
}

// MatchResult is the final result the host reports to a seat through
// MatchEnd. Outcome and Winner are "" for a draw or a halted match; Reason
// carries the host's terminal cause.
type MatchResult struct {
	Outcome        string
	Classification string
	Winner         string
	Reason         string
	StepCount      uint64
	DecisionCount  uint64
}

// MatchSeat is the optional lifecycle hook a seat implements to receive the
// match context before its first decision. An error refuses the match: the
// host treats it as a crash rather than letting a seat play without the
// context it declared it needs.
type MatchSeat interface {
	AttachMatch(*MatchContext) error
}

// MatchEnd is the optional lifecycle hook called once after the match ends,
// on the host's match goroutine. It must not block the match.
type MatchEnd interface {
	EndMatch(MatchResult)
}

// AttachMatch attaches ctx to s when s implements MatchSeat; otherwise it is
// a no-op. The host calls it once per seat after the live engine exists.
func AttachMatch(s seat.Seat, ctx *MatchContext) error {
	if m, ok := s.(MatchSeat); ok {
		return m.AttachMatch(ctx)
	}
	return nil
}

// EndMatch reports result to s when s implements MatchEnd; otherwise it is a
// no-op.
func EndMatch(s seat.Seat, result MatchResult) {
	if m, ok := s.(MatchEnd); ok {
		m.EndMatch(result)
	}
}
