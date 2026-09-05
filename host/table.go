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
type Deck struct {
	Name  string
	Cards []*cards.Card
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
}

var ErrNotFound = errors.New("host: not found")

func (c TableConfig) validate(load func(string) (Deck, error)) error {
	switch {
	case c.ID == "":
		return fmt.Errorf("host: table has no id")
	case c.Seats < 1 || c.Seats > 8:
		return fmt.Errorf("host: table %s: seats %d, want 1..8", c.ID, c.Seats)
	case len(c.Decks) == 0:
		return fmt.Errorf("host: table %s: no decks", c.ID)
	case c.Spectator != view.Public && c.Spectator != view.Omniscient:
		return fmt.Errorf("host: table %s: spectator visibility must be public or omniscient", c.ID)
	}
	for _, d := range c.Decks {
		if _, err := load(d); err != nil {
			return fmt.Errorf("host: table %s: deck %q: %w", c.ID, d, err)
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
		Spectator: t.cfg.Spectator.String(), State: t.state, Match: t.k, Perpetual: t.cfg.Perpetual}
}

func (t *table) setState(s string) {
	t.mu.Lock()
	t.state = s
	t.mu.Unlock()
}
