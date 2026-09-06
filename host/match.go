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
	"github.com/adams-shaun/gorge/state"
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
	// slots is the actual []seat.Seat the current match built in play().
	// Registry methods that must reach a per-seat *HumanSeat (Pending,
	// SubmitIntent, Task M2b-2) go through it. Installed once at the top
	// of play(), never reassigned, so a resolved *HumanSeat stays stable
	// for the match's whole lifetime.
	slots []seat.Seat

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
	cfg := rules.Config{Seed: seed, Names: names, Decks: decks, Tokens: r.opts.Tokens, Mulligans: c.Mulligans}
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
		Head: m.head, Events: events, Turns: m.e.G.Turn, Reason: m.reason, Mulligans: m.cfg.Mulligans}
}

// defaultSeats is PL-14: one bot per seat, seeded from the match seed.
func defaultSeats(names []string, seed uint64) []seat.Seat {
	out := make([]seat.Seat, len(names))
	for i := range names {
		out[i] = seat.NewBot(seed ^ uint64(i+1))
	}
	return out
}

// parkedDecision is the outcome of installing (parking) the seat that owns one
// pending decision, done — as the loop's structure now requires — before that
// decision is ever published. For a bot seat the Decision runs synchronously
// in parkSeat and in/err are already resolved; for a human seat park installed
// the answerable slot and answer() blocks until a SubmitIntent (or ctx/timeout
// caretaker) arrives. p is the decision's owner seat, kept for the crash line.
type parkedDecision struct {
	p   state.PlayerID
	hs  *parking
	in  decision.Intent
	err error
}

// answer returns the parked decision's intent: immediately for a bot, after
// blocking on the human seat's parked slot otherwise — the same intent/error
// pair the old loop got straight out of seat.S Decide.
func (pd *parkedDecision) answer() (decision.Intent, error) {
	if pd.hs != nil {
		return pd.hs.await()
	}
	return pd.in, pd.err
}

// parkedData is the projected shape of one pending decision, split out of the
// park step so the projection — which touches the live engine — can be held
// under the match's exclusive lock while the seat step runs without it.
type parkedData struct {
	p  state.PlayerID
	v  view.View
	dc decision.Decision
}

// projectNext reads the engine's current pending decision and projects the
// board for it, copying options exactly as the old loop did (Ruling FL-19
// minor: *d aliases the engine's pending, and dc.Options a slice header into
// the same backing array, so the copy is required). It touches only the
// engine — never a seat — so the loop can hold it under m.mu.Lock. That is
// the fix's lock discipline: view.Project mutates the engine's Derived cache
// (rules/layers.go Engine.active), so projecting the live engine must not run
// concurrently with a focus subscriber's own snapshot projection (which takes
// only m.mu.RLock); running it inside the Submit's exclusive section keeps
// the two from ever overlapping. Returns nil when there is no pending
// decision (game over, or a stall the caller resolves via G.Over). Call on
// the match goroutine, under m.mu.
func projectNext(m *match) *parkedData {
	d := m.e.Pending()
	if d == nil {
		return nil
	}
	v := view.Project(m.e.G, m.e, d.Player, d)
	dc := *d
	dc.Options = append([]decision.Option(nil), d.Options...)
	return &parkedData{p: d.Player, v: v, dc: dc}
}

// parkSeat installs the answerable slot for a projected decision: for a
// HumanSeat it installs the slot without blocking, for any other seat (a bot,
// or an embedder's blocking seat) it calls Decide. It is NEVER called under
// m.mu — a blocking seat must not hold the match mutex across a Decide — but
// by the time play calls it the next decision is already fully projected, and
// the caller publishes (fanout) only after it returns, so publish-outranks-park
// stays closed.
func parkSeat(ctx context.Context, seats []seat.Seat, pd *parkedData) *parkedDecision {
	if hs, ok := seats[pd.p].(*HumanSeat); ok {
		return &parkedDecision{p: pd.p, hs: hs.park(ctx, pd.v, pd.dc)}
	}
	in, err := seats[pd.p].Decide(ctx, pd.v, pd.dc)
	return &parkedDecision{p: pd.p, in: in, err: err}
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
	// Task M2c-1: if an embedder observes bursts, deliver the genesis burst
	// first so the sink sees the whole chain from its first event — genesis
	// goes through the same observeBurst as every Submit burst, on the match
	// goroutine under m.mu (genesis has no intent, so in is nil). A genesis
	// observation error crashes the match exactly as a per-burst one would
	// (D15).
	if r.opts.OnBurst != nil {
		if err := m.locked(func() error { return r.observeBurst(t, m, 0) }); err != nil {
			return r.crash(t, m, err)
		}
	}
	seats := r.opts.Seats(m.cfg.Names, m.seed)
	// Task M2c-2: honor the TableConfig.Humans plan — every listed slot is a
	// real person, so replace the bot that Options.Seats built for it (by
	// default defaultSeats, one bot per seat) with a fresh HumanSeat. The
	// remaining slots stay exactly what Options.Seats produced, so a pure-bot
	// table (Humans nil) is byte-identical to today. Each HumanSeat is armed
	// with its deterministic caretaker a few lines below, exactly like the
	// M2b-5 seats an embedder builds itself through the Seats option.
	for _, h := range t.cfg.Humans {
		seats[h] = NewHumanSeat()
	}
	m.mu.Lock()
	m.slots = seats
	m.mu.Unlock()
	// Task M2b-3: arm every human seat with its think budget and its
	// deterministic caretaker bot — the one defaultSeats would have built
	// for that slot (seed ^ slot+1), so a timed-out human decision is
	// answered by exactly the intent a pure-bot game would have logged for
	// that seat, keeping the replay byte-identical (D3). Done here, once, on
	// the match goroutine before the loop, so it never races a Decide.
	for i, s := range seats {
		if hs, ok := s.(*HumanSeat); ok {
			hs.configure(r.opts.ThinkTimeout, seat.NewBot(m.seed^uint64(i+1)))
		}
	}
	maxIntents := r.opts.MaxIntents
	if maxIntents == 0 {
		maxIntents = defaultMaxIntents
	}
	// parked is the decision currently awaiting its answer (nil before the
	// first live iteration). It is parked — installed, accept-ready — before
	// any fan-out that could publish it, so no decision is ever visible before
	// its seat can accept an answer. The first decision is parked on the first
	// live iteration, after the stop/over checks, so that if parking it blocks
	// (a seat that never answers, ctx-cancelled by Close) and that seat then
	// errors, the error is surfaced as a crash rather than masked by a stop
	// abort that raced the first park.
	var parked *parkedDecision
	var before int
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
		if n >= maxIntents {
			return r.crash(t, m, fmt.Errorf("did not terminate after %d intents (turn %d)", n, m.e.G.Turn))
		}
		// Park the decision if this is the first live iteration, or the
		// previous iteration's end-of-loop park produced nothing because the
		// game just ended (the Over check above would already have caught
		// that; a non-over nil here is the old "engine stalled" crash). The
		// projection runs under the exclusive lock so a focus subscriber
		// cannot project the live engine concurrently; parkSeat — which may
		// call a blocking Decide — runs without holding m.mu.
		if parked == nil {
			var data *parkedData
			err := m.locked(func() error {
				data = projectNext(m)
				return nil
			})
			if err != nil {
				return r.crash(t, m, err)
			}
			if data == nil {
				return r.crash(t, m, fmt.Errorf("engine stalled: game not over and no decision pending"))
			}
			parked = parkSeat(ctx, seats, data)
		}
		// Await the answer to the parked decision (parked at the first live
		// iteration or at the end of the previous one). A bot seat resolved
		// synchronously in parkSeat and returns immediately; a human seat
		// parks here on its slot until a SubmitIntent arrives or ctx/timeout
		// fires the caretaker.
		in, err := parked.answer()
		if err != nil {
			return r.crash(t, m, fmt.Errorf("seat %d: %w", parked.p, err))
		}
		var next *parkedDecision
		var nextData *parkedData
		err = m.locked(func() error {
			before = len(m.e.L.Events)
			if err := m.e.Submit(in); err != nil {
				return fmt.Errorf("intent %d rejected: %w", n, err)
			}
			m.afterSubmit(before)
			if err := r.afterBurst(t, m, before); err != nil { // Tasks 11, 12
				return fmt.Errorf("persist: %w", err)
			}
			// Still exclusive: project the engine's NEXT decision (nil when the
			// game just ended) so a focus subscriber, which projects the live
			// engine under RLock to build its snapshot, can never run that
			// projection concurrently with this one (view.Project writes the
			// Derived cache). The old loop got the same serialization because
			// its projection immediately preceded this Submit; this keeps it
			// now that the park happens after the Submit.
			nextData = projectNext(m)
			return nil
		})
		if err != nil {
			return r.crash(t, m, err)
		}
		// Install the next decision's answerable slot OUTSIDE the match lock:
		// parkSeat may call a blocking Decide (an embedder's seat), and the
		// match mutex must never be held across one. Publishing happens only
		// after this, so the park-before-publish ordering still holds.
		if nextData != nil {
			next = parkSeat(ctx, seats, nextData)
		}
		// Park the engine's NEXT decision BEFORE publishing it: the seat that
		// owns it is now accept-ready, so the fan-out below cannot expose a
		// decision no seat is waiting to answer — the race defect B fixes.
		// When the game just ended, next stays nil and the top-of-loop Over
		// check finishes. This ordering, on the match goroutine, holds for
		// every consumer of the published state.
		parked = next
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
	r.onMatchEnd(t, m)      // Tasks 10, 12
	r.observeMatchEnd(t, m) // Task M2c-1
	return protocol.MatchFinished
}

// abort records a match cut short by Close.
func (r *Registry) abort(m *match) string {
	m.mu.Lock()
	m.state = protocol.MatchAborted
	m.head = m.e.L.Head()
	m.mu.Unlock()
	r.onMatchEnd(m.table, m)
	r.observeMatchEnd(m.table, m) // Task M2c-1
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
	r.observeMatchEnd(t, m) // Task M2c-1
	return protocol.MatchCrashed
}
