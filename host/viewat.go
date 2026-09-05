package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/view"
)

// ErrBeyondHead says a requested seq is past the match's last event; Head
// is the last valid seq, which the http layer returns with a 409.
type ErrBeyondHead struct{ Head uint64 }

func (e ErrBeyondHead) Error() string { return fmt.Sprintf("host: seq beyond head %d", e.Head) }

// ViewAt is the board as of event seq (inclusive) in the table's
// visibility. For a live match it uses the turn-start snapshots; a finished
// match (Task 12) replays from its files.
//
// The replay viewAt does (up to a turn's worth of Submit calls) can run
// long enough to matter, so this takes the read lock only to copy what it
// needs — the log (m.e.L.Events/Intents are live, appended-to slices the
// match loop still owns, so a bare pointer would race; Clone gives an
// independent copy) and the (append-only, never-mutated-in-place) snapshot
// list — then computes outside the lock, never blocking a concurrent
// Submit for longer than those two copies take.
func (r *Registry) ViewAt(id TableID, k int, seq uint64) (view.View, error) {
	t, m, err := r.lookup(id, k)
	if err != nil {
		return view.View{}, err
	}
	m.mu.RLock()
	// The whole log, not a prefix to seq: Log.Clone (Task 1) is a flat
	// whole-log deep copy with no truncation parameter, and computing a
	// correct prefix here would mean re-doing Clone's own per-event
	// IDs/Pairs copying by hand instead of reusing it, for a saving that
	// is usually small (most ViewAt calls land near head anyway) and is
	// freed the moment this call returns — unlike a stored snapshot
	// (FL-44), this copy never outlives the request.
	l := m.e.L.Clone()
	snaps := append([]snapshot(nil), m.snaps...)
	cfg, vis := m.cfg, t.cfg.Spectator
	m.mu.RUnlock()
	return viewAt(cfg, l, snaps, seq, vis)
}

// Events is every redacted, described event from since (inclusive) to the
// head, in chain order. Redaction is against the state at head (PL-15),
// which holds because g is cloned at the tail's own head — RedactEventsFor
// and Describe read "the state as of the last event in evs".
//
// since is caller-controlled and since=0 on a long match is the whole log,
// so (fix round 1, FL-42) this takes the read lock only long enough to
// clone g (state.Game.Clone) and copy the events tail, then formats
// outside the lock — the same discipline as ViewAt, now that eventBodies
// (host/fanout.go) takes a state/events pair instead of a live match.
func (r *Registry) Events(id TableID, k int, since uint64) ([]protocol.EventBody, error) {
	t, m, err := r.lookup(id, k)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	n := uint64(len(m.e.L.Events))
	if n == 0 {
		m.mu.RUnlock()
		return nil, ErrBeyondHead{Head: 0}
	}
	if since >= n {
		head := n - 1
		m.mu.RUnlock()
		return nil, ErrBeyondHead{Head: head}
	}
	g := m.e.G.Clone()
	evs := append([]events.Event(nil), m.e.L.Events[since:]...)
	vis := t.cfg.Spectator
	m.mu.RUnlock()
	return eventBodies(vis, g, evs), nil
}

// lookup finds a table's live or retained match, loading a finished match
// from disk when only its sidecar is known (Task 12). The last loaded
// match is cached per table so a DVR session stepping through a finished
// match does not replay it per request.
func (r *Registry) lookup(id TableID, k int) (*table, *match, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return nil, nil, ErrNotFound
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.cur != nil && t.cur.k == k {
		return t, t.cur, nil
	}
	for _, m := range t.history {
		if m.k == k {
			return t, m, nil
		}
	}
	if t.loaded != nil && t.loaded.k == k {
		return t, t.loaded, nil
	}
	for _, sc := range t.archived {
		if sc.Match == k {
			m, err := r.loadArchived(t, sc)
			if err != nil {
				return nil, nil, err
			}
			t.loaded = m
			return t, m, nil
		}
	}
	return nil, nil, ErrNotFound
}

// loadArchived rebuilds enough of a finished match to serve ViewAt/Events:
// the log from its files and a Config from the sidecar's names and decks.
// The engine is the replay's end state, so head views are exact and mid
// views take the genesis path of viewAt (no snapshots on disk).
func (r *Registry) loadArchived(t *table, sc sidecar) (*match, error) {
	l, err := readLog(r.opts.Dir, t.cfg.ID, sc.Match)
	if err != nil {
		return nil, err
	}
	decks := make([][]*cards.Card, len(sc.Decks))
	for i, dn := range sc.Decks {
		d, err := r.opts.LoadDeck(dn)
		if err != nil {
			return nil, fmt.Errorf("host: %s/%d: deck %q: %w", t.cfg.ID, sc.Match, dn, err)
		}
		decks[i] = d.Cards
	}
	cfg := rules.Config{Seed: sc.Seed, Names: sc.Names, Decks: decks, Tokens: r.opts.Tokens}
	e, err := replay.Replay(l, cfg)
	if err != nil {
		return nil, fmt.Errorf("host: %s/%d does not replay: %w", t.cfg.ID, sc.Match, err)
	}
	return &match{table: t, k: sc.Match, seed: sc.Seed, cfg: cfg, seats: sc.Seats, decks: sc.Decks, e: e,
		bounds: boundsOf(l.Events), turnStarts: turnStartsIn(l.Events, 0), state: sc.State, result: sc.Result,
		winner: sc.Winner, head: sc.Head}, nil
}

// viewAt is PL-1: find the last intent boundary j with bounds[j] <= seq+1,
// reach it from the nearest snapshot (or genesis) by re-submitting the
// recorded intents, apply the rest of that burst's events onto the clone's
// own game, and project. Zones, life, damage, counters and the stack are
// exact at every seq; derived P/T from continuous effects and the pending
// tray are as of the burst's start (at most one resolution stale).
//
// A reader must never crash (D15's philosophy, extended to readers by
// fix round 1, FL-43): boundsOf already keeps a crashed match's poison
// intent (the one Submit recorded right before a panic, spec D15) out of
// the replay range, but the deferred recover below is the backstop for
// any other way replaying a hand-built, corrupted, or future-buggy log
// could panic — it turns that into a plain error instead.
func viewAt(cfg rules.Config, l *events.Log, snaps []snapshot, seq uint64, vis view.Visibility) (v view.View, err error) {
	n := uint64(len(l.Events))
	if n == 0 {
		return view.View{}, ErrBeyondHead{Head: 0}
	}
	if seq >= n {
		return view.View{}, ErrBeyondHead{Head: n - 1}
	}
	defer func() {
		if p := recover(); p != nil {
			v, err = view.View{}, fmt.Errorf("host: viewAt(seq=%d) panicked: %v", seq, p)
		}
	}()
	bounds := boundsOf(l.Events)
	if len(bounds) == 0 {
		return view.View{}, fmt.Errorf("host: no intent boundaries found in a %d-event log", n)
	}
	j := 0
	for j+1 < len(bounds) && bounds[j+1] <= seq+1 {
		j++
	}
	// Nearest snapshot at or before boundary j; snaps are in ascending order.
	var e *rules.Engine
	from := 0
	for i := len(snaps) - 1; i >= 0; i-- {
		if snaps[i].intent <= j {
			e = snaps[i].e.Clone()
			from = snaps[i].intent
			break
		}
	}
	if e == nil {
		var err error
		e, err = replay.ReplayTo(l, cfg, j)
		if err != nil {
			return view.View{}, fmt.Errorf("host: replay to boundary %d: %w", j, err)
		}
		from = j
	}
	for i := from; i < j && i < len(l.Intents); i++ {
		if err := e.Submit(l.Intents[i]); err != nil {
			return view.View{}, fmt.Errorf("host: replay intent %d: %w", i, err)
		}
	}
	if got := uint64(len(e.L.Events)); got != bounds[j] {
		return view.View{}, fmt.Errorf("host: replay reached seq %d, boundary %d is %d", got, j, bounds[j])
	}
	for s := bounds[j]; s <= seq; s++ {
		events.Apply(e.G, l.Events[s])
	}
	return view.ProjectFor(e.G, e, view.NoSeat, vis, nil), nil
}
