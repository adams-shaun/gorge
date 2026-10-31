package game

import "github.com/adams-shaun/gorge/engine/cards"

// Format is a game's construction format.
type Format int

const (
	FormatConstructed Format = iota
	FormatCommander
)

// Config builds a game. A replay must be given the values the live game had;
// only the guard fields and ExpectedEvents may differ.
type Config struct {
	Seed uint64
	// Names is each seat's deck identity, which reaches event text.
	// PlayerNames is the display name, falling back to Names.
	Names, PlayerNames []string
	Decks, Sideboards  [][]*cards.Card
	Archetypes         []string
	Tokens             map[string]*cards.Card
	NameUniverse       *cards.Universe
	// NameUniverseNames pins a persisted game's offered names; a live game
	// leaves it nil and derives them from NameUniverse.
	NameUniverseNames []string
	Mulligans         int
	WindowDiagnostics bool
	// DisablePaidManaCensus narrows the mana census to free-to-tap sources.
	DisablePaidManaCensus bool
	Format                Format
	StartingLife          int32 // 0 is the default for the format
	Commanders            [][]int
	StartChooser          *int // the seat posed CR 103.1's choice instead of the toss winner
	// GuardDisabled means the host's per-turn guard is off, so the engine's
	// livelock watcher is too.
	GuardDisabled bool
	// NoObjectCap leaves the engine's watcher at its defaults with no bound on
	// the board's size, for rebuilding a recording that may have grown past it.
	NoObjectCap bool
	// ExpectedEvents is a log capacity hint.
	ExpectedEvents int
	// Key is an engine-specific replay key the host records beside the log.
	Key string
}
