// Package game is what a host needs from a running game: the board and log it
// projects from, the decision it poses, the answer it submits. Each engine
// implements it; nothing here names one.
package game

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// Game is one running game. One goroutine calls Submit. Between Submits, Board
// and Log may be read concurrently; Chars may reuse scratch space between
// calls, so its readers exclude each other.
type Game interface {
	// Board is read-only and changes only through Submit.
	Board() *state.Game
	// Chars answers derived facts over Board.
	Chars() view.Chars
	// Log holds the events and accepted intents so far.
	Log() *events.Log
	// Pending is nil when the game is over or stalled.
	Pending() *decision.Decision
	// Submit answers Pending. A refusal leaves the pending decision intact.
	Submit(in decision.Intent) error
	// Fork returns an independent copy at this point.
	Fork() Game
	Caps() Caps
}

// Caps says what a Game can do beyond the interface. A false field is an
// honest absence: the host refuses or degrades, it never substitutes.
type Caps struct {
	Engine        string // the name Engine.Name reports
	SubBurstViews bool   // events.Apply of a logged event reproduces the board at that event
	ManualMana    bool   // decisions may offer mana abilities and payment windows
	SearchRoots   bool   // a search bot can deal its root from this game
	StartingAsk   bool   // CR 103.1's choice is posed
}

// PaymentPlanner is a Game that builds Decision.PaymentActions on request and
// can prove an offered witness against the live state.
type PaymentPlanner interface {
	EnsurePaymentActions()
	EnsureTargetHints()
	ValidateCastPayment(p state.PlayerID, cast decision.PlannedCast, plan decision.PaymentPlan) error
	ValidateCastAnnounce(p state.PlayerID, cast decision.PlannedCast) error
}

// PotentialProver is a game that can prove which of a seat's potential plays it
// cannot pay (rules.Engine.EnsurePotentialProofs).
type PotentialProver interface {
	EnsurePotentialProofs()
}

// Facts is what an observation reports of an object's characteristics.
type Facts struct {
	Name             string
	Types            []string
	Colors           string // WUBRG letters
	Keywords         []string
	Power, Toughness int32
}

// Observed is a Game a seat can read beyond Chars, when no gorge engine stands behind
// it: what a v2 observation names of an object, and how many times it has changed
// zones, since this game's log does not rebuild its board.
type Observed interface {
	HasKeyword(id state.ObjID, kw string) bool
	// ObservedAbilities lists id's non-mana activated abilities in a fixed order.
	ObservedAbilities(p state.PlayerID, id state.ObjID) []decision.Option
	Facts(id state.ObjID) Facts
	// HostTriggerAmount is the causing event's magnitude for a stack ability that
	// reads one, or false.
	HostTriggerAmount(id state.ObjID) (int32, bool)
	ZoneMoves(id state.ObjID) uint32
}

// Keyed is a game whose replay is pinned to what it was built from. The host records
// the key as Config.Key beside the log.
type Keyed interface {
	EngineKey() string
}

// Stalled is a Game that can end without a result and say why.
type Stalled interface {
	// StallReason is "" unless Pending is nil and Board().Over is false.
	StallReason() string
}

// Engine builds games. Replay, ReplayTo and Walk verify the rebuilt events
// against l; on a *Divergence the Game is still returned, at the point it
// stopped. n counts recorded intents and clamps to the log's range.
type Engine interface {
	Name() string
	// Caps is what this engine's games report, for a host deciding before it
	// has one.
	Caps() Caps
	// Accepts says why c cannot play here, naming the limit or card, or nil.
	Accepts(c Config) error
	// New builds a game through genesis and the first Advance.
	New(c Config) (Game, error)
	Replay(l *events.Log, c Config) (Game, error)
	ReplayTo(l *events.Log, c Config, n int) (Game, error)
	// Walk calls visit at each intent boundary up to n, before that intent is
	// submitted, and once at the last. An error from visit stops the walk.
	Walk(l *events.Log, c Config, n int, visit func(g Game, i int) error) (Game, error)
}
