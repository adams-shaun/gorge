package host

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// TableID is stable for the life of a registry directory; match indices
// under it increment from 1.
type TableID string

// Deck is a resolved deck list: the name a seat is called and the cards it
// is dealt. Options.LoadDeck produces it; the host never reads files for
// decks itself.
//
// Commanders is the flat indices into Cards that are this deck's
// commanders, in the deck file's order (one today — deck.File.Commander);
// the engine places those objects in the command zone at genesis. nil for
// a constructed deck. The loader — which owns the deck files — computes it
// (deck.File.CommanderIndex, validated by deck.ValidateCommander), so the
// host never guesses which card a deck means by its commander.
type Deck struct {
	Name  string
	Cards []*cards.Card

	Commanders []int
}

// Format is a table's construction format, named on the wire like a
// Visibility: "constructed" is the zero value and the only value every
// table before m39 carried; "commander" runs the CR 903 rules the engine
// gates on its own rules.Format. The host-local enum exists so TableConfig
// — persisted verbatim in tables.json — reads as names, the same
// convention Spectator follows; newMatch converts it to the engine's int
// format at match build, so the two never need to grow in lockstep.
type Format uint8

const (
	// FormatConstructed is a 20-life game with no command zone — the zero
	// value, so a tables.json written before the field existed loads as
	// constructed and validates exactly as it always did.
	FormatConstructed Format = iota
	// FormatCommander is a Commander game: 40 life (CR 903.6), a command
	// zone built from each seat's deck's commanders, and the CR 903.8 tax
	// and CR 903.10 damage clock — whatever the engine's FormatCommander
	// gates on.
	FormatCommander
)

var formatNames = [...]string{"constructed", "commander"}

// String is the wire name; an out-of-range value prints "unknown", the
// same total shape as state.Step.String.
func (f Format) String() string {
	if int(f) < len(formatNames) {
		return formatNames[f]
	}
	return "unknown"
}

// ParseFormat is String's inverse for flags and table configs.
func ParseFormat(s string) (Format, error) {
	for i, n := range formatNames {
		if n == s {
			return Format(i), nil
		}
	}
	return 0, fmt.Errorf("host: unknown format %q (want constructed or commander)", s)
}

// MarshalText/UnmarshalText make a Format its name in JSON, so a table
// configuration on disk reads "commander", not 1.
func (f Format) MarshalText() ([]byte, error) { return []byte(f.String()), nil }

func (f *Format) UnmarshalText(b []byte) error {
	p, err := ParseFormat(string(b))
	if err != nil {
		return err
	}
	*f = p
	return nil
}

// TableConfig is everything a table needs; it is persisted verbatim in
// tables.json, so a table's whole history is reproducible from it.
type TableConfig struct {
	ID    TableID  `json:"id"`
	Name  string   `json:"name"`
	Seats int      `json:"seats"`
	Decks []string `json:"decks"` // deck names for Options.LoadDeck; seat i of match k plays Decks[(i+k)%len]
	Seed  uint64   `json:"seed"`
	// Pace is the sleep after every decision; 0 plays as fast as possible.
	Pace      time.Duration   `json:"pace"`
	Spectator view.Visibility `json:"spectator"` // Public or Omniscient
	Perpetual bool            `json:"perpetual"`
	// Humans is the set of slot indices that are real people: a non-nil,
	// non-empty value makes the table single-shot and seats every listed slot
	// with a HumanSeat (driven from outside via Pending/SubmitIntent) instead
	// of a bot. nil (the default) means all bots — byte-identical to today.
	// The bot slots are the complement: every index in [0, Seats) that Humans
	// does not list, so a human index is disjoint from the bots by
	// construction once it is in range. A table with Humans must not also be
	// Perpetual (validation rejects that combination); a human-seated table
	// always ends at game over.
	Humans []int `json:"humans,omitempty"`
	// Mulligans is the London mulligan allowance: each player may take up to
	// this many mulligans in the pre-game round between the opening deal and
	// turn 1 (rules.Config.Mulligans). 0 — the zero value, and what every
	// table before M2e-5 played with — skips the round entirely, so a table
	// that never sets it behaves byte-identically to today. It travels on the
	// same TableConfig that seed/decks/names travel on, so both the live
	// match and a restart's replays rebuild the same rules.Config (R-8.4: the
	// replay's Config must carry it too, or a match played with a round stops
	// replaying). Negative values are rejected by validate.
	Mulligans int `json:"mulligans,omitempty"`
	// Format names the table's construction format: constructed (the zero
	// value, and every table before m39) or commander. A commander table
	// plays the CR 903 rules — 40 life (CR 903.6), a command zone built
	// from each seat's deck's commanders, the CR 903.8 tax and the CR
	// 903.10 clock — so every deck it deals must itself name a commander
	// (validate enforces it through the loaded Deck). Named on the wire
	// like Spectator; a tables.json written before the field existed loads
	// as constructed.
	Format Format `json:"format,omitempty"`
	// StartingLife overrides a game's opening life total; 0 (the zero
	// value) means the format's default — 20 constructed, 40 for a
	// commander table (CR 903.6) — so a commander table that never sets it
	// still plays 40. newMatch resolves the default once, on the copy it
	// builds the match from, and the resolved value travels on every
	// rules.Config and on its sidecar, so a replay rebuilds the life the
	// match actually played with.
	StartingLife int32 `json:"starting_life,omitempty"`
}

var ErrNotFound = errors.New("host: not found")

func (c TableConfig) validate(load func(string) (Deck, error)) error {
	switch {
	case c.ID == "":
		return fmt.Errorf("host: table has no id")
	case c.Seats < 1 || c.Seats > 8:
		return fmt.Errorf("host: table %s: seats %d, want 1..8", c.ID, c.Seats)
	case c.Mulligans < 0:
		return fmt.Errorf("host: table %s: mulligans %d, want >= 0 (0 disables the London round)", c.ID, c.Mulligans)
	case len(c.Decks) == 0:
		return fmt.Errorf("host: table %s: no decks", c.ID)
	case c.Spectator != view.Public && c.Spectator != view.Omniscient:
		return fmt.Errorf("host: table %s: spectator visibility must be public or omniscient", c.ID)
	}
	// Task M2c-2: a human-seated table is single-shot by definition, so an
	// explicit Perpetual: true alongside Humans is a caller contradicting
	// itself. Reject rather than silently force single-shot — silently
	// ignoring an explicit Perpetual is the kind of thing discovered in
	// production.
	if c.Perpetual && len(c.Humans) > 0 {
		return fmt.Errorf("host: table %s: perpetual and humans cannot be combined; a human-seated table is single-shot", c.ID)
	}
	// Human indices must be in range, unique, and — because every bot slot is
	// the complement of this set — thereby disjoint from the bot slots.
	// Reject out of range before the duplicate pass so a bad index can never
	// reach a map or slot access (no panic, no half-created table).
	if c.Humans != nil {
		for i, h := range c.Humans {
			if h < 0 || h >= c.Seats {
				return fmt.Errorf("host: table %s: human seat %d (element %d) out of range 0..%d", c.ID, h, i, c.Seats-1)
			}
		}
		seen := make(map[int]bool, len(c.Humans))
		for _, h := range c.Humans {
			if seen[h] {
				return fmt.Errorf("host: table %s: duplicate human seat %d", c.ID, h)
			}
			seen[h] = true
		}
	}
	for _, d := range c.Decks {
		dd, err := load(d)
		if err != nil {
			return fmt.Errorf("host: table %s: deck %q: %w", c.ID, d, err)
		}
		// A commander table dealt a deck that names no commander is a
		// config error, not a game: the seat would play without a command
		// zone, and a commander index out of range would be silently
		// skipped by the engine's New. Reject both here, at the boundary
		// where the table is configured, so a half-started commander
		// table is impossible rather than merely unlikely.
		if c.Format == FormatCommander {
			if len(dd.Commanders) == 0 {
				return fmt.Errorf("host: table %s: commander table deck %q names no commander", c.ID, d)
			}
			for _, ci := range dd.Commanders {
				if ci < 0 || ci >= len(dd.Cards) {
					return fmt.Errorf("host: table %s: deck %q commander index %d out of range 0..%d for %d cards", c.ID, d, ci, len(dd.Cards)-1, len(dd.Cards))
				}
			}
		}
	}
	return nil
}

// table is one registry entry and the goroutine that plays it. started is
// guarded by Registry.mu — Start and Wait both read/write it while already
// holding that lock (registry.go), not t.mu. mu guards every field from
// state down to done; the run loop and every reader take it. fanMu and
// lastLine below that are a deliberate exception — see their own docs.
type table struct {
	cfg TableConfig

	// started is guarded by Registry.mu, not mu below. See the struct doc.
	started bool

	mu    sync.RWMutex
	state string // protocol.Table*
	// reason is the halt error's message, set by Registry.halt; empty
	// unless state is TableHalted. Task 13 surfaces it on the wire
	// (protocol.TableInfo has no field for it yet); kept internal for now.
	reason  string
	k       int    // index of the current or most recent match; 0 before any
	cur     *match // the live match, or nil
	history []*match
	// archived holds finished matches known only from disk, ascending by
	// match index (Task 12). They are served from their files, never kept
	// in memory.
	archived []sidecar
	// loaded caches the last archived match rebuilt from disk, so a DVR
	// session stepping through a finished match does not replay it per
	// request (Task 12).
	loaded *match
	stop   chan struct{} // closed by Registry.Close
	done   chan struct{} // closed when the run loop exits

	// fanMu serialises a focus Subscribe's snapshot build+push (session.go)
	// against the match loop's own fan-out push loops (fanout/onMatchStart/
	// onMatchEnd in fanout.go), so a client that joins a live table can
	// never receive a stale snapshot after events newer than it (Ruling
	// FL-30). push itself never blocks, so holding fanMu across a push
	// loop never parks the match loop on a client — the only contention is
	// a subscribing HTTP goroutine, held only as long as it takes to build
	// and push one frame set.
	fanMu sync.Mutex
	// lastLine is the most recent non-empty transcript line, carried
	// forward for the overview widget's Last field across bursts whose own
	// events are all line-less (e.g. an all-clock_tick burst). Touched
	// only by this table's own run/play goroutine — fanout, onMatchStart
	// and onMatchEnd all run there, one at a time, never concurrently with
	// each other or with anything else — so it needs no lock of its own.
	lastLine string
}

func newTable(cfg TableConfig) *table {
	return &table{cfg: cfg, state: protocol.TableIdle, stop: make(chan struct{}), done: make(chan struct{})}
}

func (t *table) info() protocol.TableInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return protocol.TableInfo{ID: string(t.cfg.ID), Name: t.cfg.Name, Seats: t.cfg.Seats,
		Spectator: t.cfg.Spectator.String(), State: t.state, Match: t.k, Perpetual: t.cfg.Perpetual,
		Format: t.cfg.Format.String()}
}

// singleShot reports whether the run loop should stop after exactly one
// match instead of autoplaying the next. A non-perpetual table is single-
// shot; so is a human-seated table (Task M2c-2) — a real person must not be
// pulled into a fresh match behind their back. AddTable already rejects
// Perpetual+Humans, so in practice the Humans term makes singleShot true for
// the same table a non-Perpetual flag would; keeping both explicit documents
// the intent and stays correct even if the two flags ever disagree (e.g. a
// hand-edited persisted table).
func (c TableConfig) singleShot() bool {
	return !c.Perpetual || len(c.Humans) > 0
}

func (t *table) setState(s string) {
	t.mu.Lock()
	t.state = s
	t.mu.Unlock()
}
