package host

import (
	"fmt"

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
	l := m.e.L.Clone()
	snaps := append([]snapshot(nil), m.snaps...)
	cfg, vis := m.cfg, t.cfg.Spectator
	m.mu.RUnlock()
	return viewAt(cfg, l, snaps, seq, vis)
}

// Events is every redacted, described event from since (inclusive) to the
// head, in chain order. Redaction is against the state at head (PL-15).
//
// This holds the read lock for the eventBodies call: that helper (Task 10,
// fanout.go) is shared with the burst fan-out and its contract there is
// "called with m.mu held", reading m.e.G/m.e.L live. Unlike ViewAt's
// replay, it does no engine work — formatting and redaction only — and
// that helper's file is outside this task's scope, so this keeps its
// existing, already-reviewed locking convention rather than forking it.
func (r *Registry) Events(id TableID, k int, since uint64) ([]protocol.EventBody, error) {
	t, m, err := r.lookup(id, k)
	if err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := uint64(len(m.e.L.Events))
	if n == 0 {
		return nil, ErrBeyondHead{Head: 0}
	}
	if since >= n {
		return nil, ErrBeyondHead{Head: n - 1}
	}
	return r.eventBodies(t, m, int(since)), nil
}

// lookup finds a table's live or retained match. Task 12 teaches it to
// load finished matches from disk.
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
	return nil, nil, ErrNotFound
}

// viewAt is PL-1: find the last intent boundary j with bounds[j] <= seq+1,
// reach it from the nearest snapshot (or genesis) by re-submitting the
// recorded intents, apply the rest of that burst's events onto the clone's
// own game, and project. Zones, life, damage, counters and the stack are
// exact at every seq; derived P/T from continuous effects and the pending
// tray are as of the burst's start (at most one resolution stale).
func viewAt(cfg rules.Config, l *events.Log, snaps []snapshot, seq uint64, vis view.Visibility) (view.View, error) {
	n := uint64(len(l.Events))
	if n == 0 {
		return view.View{}, ErrBeyondHead{Head: 0}
	}
	if seq >= n {
		return view.View{}, ErrBeyondHead{Head: n - 1}
	}
	bounds := boundsOf(l.Events)
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
