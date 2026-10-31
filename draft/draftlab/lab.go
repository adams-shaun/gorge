package draftlab

import (
	"fmt"

	_ "github.com/adams-shaun/gorge/bots/all" // link every built-in hosted policy
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// The run defaults, cmd/draftsim's watchdogs and London mulligan round:
// a limited game that reaches a cap is a watchdog draw, and a 40-card
// deck with ~17 lands wants one extra London round.
const (
	defaultMaxTurns   = 150
	defaultMaxIntents = 40000
	defaultMulligans  = 2
)

// Options are a lab's run options. They shape how (future) games run, never
// what a result is: Workers/Verify are run knobs, MaxTurns/MaxIntents cap a
// game as a watchdog draw, and Mulligans sets the London mulligan round.
type Options struct {
	// AllowDecklistBots admits Player policies whose bots.Info.Env is true
	// (they read every seat's real decklist). The lab seats bots only, so
	// this is a bot-only opt-in, by design.
	// Default false.
	AllowDecklistBots bool
	// Workers is how many games run at once (E5-c2); 0 or 1 = serial. It
	// never changes a result.
	Workers int
	// Verify replays every game's chain (E5-c2); a mismatch is an error.
	Verify bool
	// MaxTurns caps a game's turns; 0 = 150 (draftsim).
	MaxTurns int
	// MaxIntents caps a game's intents; 0 = 40000 (draftsim).
	MaxIntents int
	// MaxObjs arms the engine's board-size cap (rules.LoopGuard.MaxObjs):
	// a game whose object arena passes it stops as a stall instead of
	// growing without bound (a token-doubling board such as Doubling Season
	// with Extravagant Replication doubles every upkeep until the turn cap,
	// and one such game held 20 GB). 0 = off, the historical default.
	MaxObjs int
	// Mulligans is the London mulligan round per seat; 0 = 2 (draftsim);
	// < 0 = none.
	Mulligans int
	// RecordTape keeps each game's accepted intents in GameResult.Tape so
	// the game re-drives with no player policy (ReplayTape,
	// VerifyEventTapes). Off by default.
	RecordTape bool
	// OnEngine, when set, is handed each finished game's engine (its event
	// log included) before the result is built. A spike hook for analysis
	// tooling (cmd/cardval's note census); it must not mutate the engine.
	OnEngine func(seed uint64, eng *rules.Engine)
}

// Lab is one opened composition surface: a read-only card corpus plus the
// lab's options. It is immutable after Open and safe to call from several
// goroutines; every method is a pure function of its inputs and seed.
type Lab struct {
	reg  *cards.Registry
	opts Options
}

// Open opens the compiled card corpus at corpusDir (cards.SharedCorpus: one
// registry per directory per process) and returns a lab over it. Options
// are validated first; a negative Workers, MaxTurns or MaxIntents is an
// error (Mulligans < 0 is legal: it means no mulligan round).
func Open(corpusDir string, o Options) (*Lab, error) {
	switch {
	case o.Workers < 0:
		return nil, fmt.Errorf("draftlab: Workers %d is negative (0 or 1 mean serial)", o.Workers)
	case o.MaxTurns < 0:
		return nil, fmt.Errorf("draftlab: MaxTurns %d is negative (0 = the default %d)", o.MaxTurns, defaultMaxTurns)
	case o.MaxIntents < 0:
		return nil, fmt.Errorf("draftlab: MaxIntents %d is negative (0 = the default %d)", o.MaxIntents, defaultMaxIntents)
	}
	reg, err := cards.SharedCorpus(corpusDir)
	if err != nil {
		return nil, fmt.Errorf("draftlab: opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", corpusDir, err)
	}
	return &Lab{reg: reg, opts: o}, nil
}

// Card resolves a card name against the lab's read-only corpus registry
// (adapters use it to describe a pack or pool to an out-of-process agent).
func (l *Lab) Card(name string) (*cards.Card, bool) {
	return l.reg.Lookup(name)
}

// Options reports the lab's options with the defaults filled: a zero
// MaxTurns/MaxIntents/Mulligans reads back as its default.
func (l *Lab) Options() Options {
	o := l.opts
	if o.MaxTurns == 0 {
		o.MaxTurns = defaultMaxTurns
	}
	if o.MaxIntents == 0 {
		o.MaxIntents = defaultMaxIntents
	}
	if o.Mulligans == 0 {
		o.Mulligans = defaultMulligans
	}
	return o
}
