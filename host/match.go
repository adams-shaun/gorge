package host

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

const defaultMaxIntents = 400000

// match is one game on a table: the engine, the intent boundaries and
// turn starts a view request needs, and the outcome. mu guards everything
// below cfg: the run loop holds it for the duration of each Submit and its
// bookkeeping; readers (ViewAt, Events, fan-out) hold it for reads only and
// never drive the engine.
type match struct {
	table *table
	k     int
	seed  uint64
	cfg   rules.Config
	seats []protocol.SeatInfo
	decks []string

	mu sync.RWMutex
	e  *rules.Engine
	// files is the live match's append-only logs; nil in memory mode and
	// after the match is archived (Task 12).
	files *matchFiles
	// persisted is the number of events confirmed appended to this match's
	// events file at the last successful persist; only that prefix is
	// durable, so a crash or kill records the head over it, never the
	// in-memory tail (fix round 1, burst atomicity). 0 in memory mode.
	persisted int
	// bounds[j] is len(e.L.Events) after j intents: the seq one past the
	// end of the j-th burst. bounds[0] is genesis plus the first Advance.
	bounds []uint64
	// turnStarts is the seq of every TurnChange so far — the DVR's ticks.
	turnStarts []uint64
	snaps      []snapshot // Task 11
	intents    int
	state      string // protocol.Match*
	result     string // "win", "draw" or ""
	winner     *uint8
	head       string
	reason     string // crash reason (Task 13)
}

// snapshot is a cloned engine at an intent boundary that began a turn.
type snapshot struct {
	intent int
	seq    uint64 // Task 12: the persisted burst boundary this snapshot lines up with on disk.
	e      *rules.Engine
}

// newMatch resolves decks, seeds and builds the engine through genesis and
// the first Advance, so the returned match is at intent boundary 0. When
// persistence is on it also opens the match's files, writes the live
// sidecar and appends the genesis events; any error halts the table.
func (r *Registry) newMatch(t *table, k int) (*match, error) {
	c := t.cfg
	seed := MatchSeed(c.Seed, k)
	names := make([]string, c.Seats)
	decks := make([][]*cards.Card, c.Seats)
	deckNames := make([]string, c.Seats)
	infos := make([]protocol.SeatInfo, c.Seats)
	for i := 0; i < c.Seats; i++ {
		dn := c.Decks[(i+k)%len(c.Decks)]
		d, err := r.opts.LoadDeck(dn)
		if err != nil {
			return nil, fmt.Errorf("host: table %s match %d: deck %q: %w", c.ID, k, dn, err)
		}
		if d.Name == "" {
			d.Name = dn
		}
		names[i], decks[i], deckNames[i] = d.Name, d.Cards, dn
		infos[i] = protocol.SeatInfo{Name: d.Name, Deck: dn, Colour: protocol.SeatColours[i%len(protocol.SeatColours)]}
	}
	cfg := rules.Config{Seed: seed, Names: names, Decks: decks, Tokens: r.opts.Tokens}
	e := rules.New(cfg)
	e.Advance()
	m := &match{table: t, k: k, seed: seed, cfg: cfg, seats: infos, decks: deckNames, e: e, state: protocol.MatchLive}
	m.bounds = []uint64{uint64(len(e.L.Events))}
	m.turnStarts = turnStartsIn(e.L.Events, 0)
	m.snapshotGenesis()
	if r.opts.Dir != "" {
		var err error
		m.files, err = openMatchFiles(r.opts.Dir, t.cfg.ID, k, r.opts.Sync)
		if err != nil {
			return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
		}
		if err := writeSidecar(r.opts.Dir, m.sidecar(), r.opts.Sync); err != nil {
			m.files.close()
			return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
		}
		if err := m.files.append(e.L.Events, nil); err != nil {
			m.files.close()
			return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
		}
		m.persisted = len(e.L.Events) // genesis is durable once appended
	}
	return m, nil
}

// turnStartsIn lists the seq of every TurnChange in evs[from:].
func turnStartsIn(evs []events.Event, from int) []uint64 {
	var out []uint64
	for _, ev := range evs[from:] {
		if ev.Kind == events.TurnChange {
			out = append(out, ev.Seq)
		}
	}
	return out
}

// locked runs fn under the write lock and releases it even if fn panics —
// a panicking Submit must not leave the mutex held, or crash (which takes
// the lock to record the failure) would deadlock the table.
func (m *match) locked(fn func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn()
}

// afterSubmit records the burst that a successful Submit just produced.
// Called with m.mu held.
func (m *match) afterSubmit(before int) {
	m.intents++
	m.bounds = append(m.bounds, uint64(len(m.e.L.Events)))
	m.turnStarts = append(m.turnStarts, turnStartsIn(m.e.L.Events, before)...)
}

// info is the sidecar/wire summary. Called with m.mu held for reading.
func (m *match) info() protocol.MatchInfo {
	return protocol.MatchInfo{Table: string(m.table.cfg.ID), Match: m.k, Seed: m.seed, Seats: m.seats,
		State: m.state, Result: m.result, Winner: m.winner, Head: m.head,
		Events: len(m.e.L.Events), Turns: m.e.G.Turn}
}

// sidecar is the on-disk summary of the match. Called with m.mu held.
// For a crashed match the summary reflects the persisted prefix, not the
// in-memory tail: crash() recorded m.head over it, and Events here is the
// persisted count (fix round 1).
func (m *match) sidecar() sidecar {
	events := len(m.e.L.Events)
	if m.files != nil && m.state == protocol.MatchCrashed {
		events = m.persisted
	}
	return sidecar{Table: string(m.table.cfg.ID), Match: m.k, Seed: m.seed, Seats: m.seats, Names: m.cfg.Names,
		Decks: m.decks, Spectator: m.table.cfg.Spectator.String(), State: m.state, Result: m.result, Winner: m.winner,
		Head: m.head, Events: events, Turns: m.e.G.Turn, Reason: m.reason}
}

// defaultSeats is PL-14: one bot per seat, seeded from the match seed.
func defaultSeats(names []string, seed uint64) []seat.Seat {
	out := make([]seat.Seat, len(names))
	for i := range names {
		out[i] = seat.NewBot(seed ^ uint64(i+1))
	}
	return out
}

// play drives m to completion, abort or crash on the table's goroutine and
// returns the final match state. A panic anywhere in a decision or Submit
// is a crash (spec D15), never a dead goroutine.
//
// ctx is the table's own context (run derives it once from t.stop, over the
// table's whole lifetime, not per match): it is the only cancellation path
// into a Seat.Decide call once the loop is blocked inside one, since t.stop
// itself is polled only between decisions (Ruling FL-17). A bot ignores ctx
// and never blocks; a disconnected human seat is expected to select on it.
func (r *Registry) play(ctx context.Context, t *table, m *match) (final string) {
	defer func() {
		if p := recover(); p != nil {
			final = r.crash(t, m, fmt.Errorf("panic: %v\n%s", p, debug.Stack()))
		}
	}()
	seats := r.opts.Seats(m.cfg.Names, m.seed)
	maxIntents := r.opts.MaxIntents
	if maxIntents == 0 {
		maxIntents = defaultMaxIntents
	}
	for n := 0; ; n++ {
		select {
		case <-t.stop:
			return r.abort(m)
		default:
		}
		// The loop is the only writer, so reading without the lock here
		// is safe; readers on other goroutines take RLock and see either
		// the state before or after the Lock section below.
		if m.e.G.Over {
			return r.finish(t, m)
		}
		d := m.e.Pending()
		if d == nil {
			return r.crash(t, m, fmt.Errorf("engine stalled: game not over and no decision pending"))
		}
		if n >= maxIntents {
			return r.crash(t, m, fmt.Errorf("did not terminate after %d intents (turn %d)", n, m.e.G.Turn))
		}
		v := view.Project(m.e.G, m.e, d.Player, d)
		// Copy the decision before handing it to the seat: *d aliases the
		// engine's own pending decision, and dc.Options a slice header
		// pointing at the same backing array (Ruling FL-19 minor) — the
		// View above already deep-copies exactly this list (view/view.go)
		// because a Seat "must not be able to corrupt the live decision
		// through it"; the raw argument here needs the same protection.
		dc := *d
		dc.Options = append([]decision.Option(nil), d.Options...)
		in, err := seats[d.Player].Decide(ctx, v, dc)
		if err != nil {
			return r.crash(t, m, fmt.Errorf("seat %d: %w", d.Player, err))
		}
		var before int
		err = m.locked(func() error {
			before = len(m.e.L.Events)
			if err := m.e.Submit(in); err != nil {
				return fmt.Errorf("intent %d rejected: %w", n, err)
			}
			m.afterSubmit(before)
			if err := r.afterBurst(t, m, before); err != nil { // Tasks 11, 12
				return fmt.Errorf("persist: %w", err)
			}
			return nil
		})
		if err != nil {
			return r.crash(t, m, err)
		}
		r.fanout(t, m, before) // Task 10
		r.opts.Sleep(t.cfg.Pace, t.stop)
	}
}

// finish records a natural end.
func (r *Registry) finish(t *table, m *match) string {
	m.mu.Lock()
	m.state = protocol.MatchFinished
	m.head = m.e.L.Head()
	if m.e.G.Draw {
		m.result = "draw"
	} else {
		m.result = "win"
		w := uint8(m.e.G.Winner)
		m.winner = &w
	}
	m.mu.Unlock()
	r.onMatchEnd(t, m) // Tasks 10, 12
	return protocol.MatchFinished
}

// abort records a match cut short by Close.
func (r *Registry) abort(m *match) string {
	m.mu.Lock()
	m.state = protocol.MatchAborted
	m.head = m.e.L.Head()
	m.mu.Unlock()
	r.onMatchEnd(m.table, m)
	return protocol.MatchAborted
}

// crash is spec D15's first half: the match is marked crashed with its
// reason. Task 13 adds the crash report, the table halt frame and tests.
// Fix round 1 (burst atomicity): the head recorded here is over the
// persisted prefix, not the in-memory log — the in-memory log can be ahead
// of disk because the crashed Submit's events never fully reached the
// files, and a sidecar naming a head/count that was never written would
// make a restart serve a log that cannot replay. m.persisted is the last
// fully-appended boundary.
func (r *Registry) crash(t *table, m *match, err error) string {
	m.mu.Lock()
	m.state = protocol.MatchCrashed
	m.reason = err.Error()
	if m.files != nil {
		m.head = m.e.L.HeadAt(m.persisted)
	} else {
		m.head = m.e.L.Head()
	}
	m.mu.Unlock()
	r.writeCrashReport(t, m, err.Error())
	r.onMatchEnd(t, m)
	return protocol.MatchCrashed
}
