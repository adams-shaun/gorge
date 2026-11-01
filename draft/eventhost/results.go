package eventhost

import (
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// gameRef is the event game attempt one host table is playing for its run.
// A series ref stays registered until its series ends: firstGame is
// the game the table's match 1 plays, so match k plays game firstGame+k-1
// with the ref's attempt for k == 1 and attempt 1 for later matches (a
// series game's attempt counter restarts after each GameEnded).
type gameRef struct {
	run     *run
	round   int
	a, b    int
	game    int
	attempt int
	series  bool
	// firstGame is the series game the table's match 1 plays; 0 on a Bo1
	// ref (whose game is game).
	firstGame int
}

// outcome is one finished game match waiting for its pairing-order place
// in the event log: flush applies outcomes only in pairing order, so a
// later series' result waits until every earlier series of the round has
// ended, and two replays log identical entry orders however the games
// finished.
type outcome struct {
	ref gameRef
	m   protocol.MatchInfo
}

// OnMatchEnd is the Host's host.OnMatchEndFunc: every event game table's
// match terminal state arrives here. It ignores non-event tables, aborted
// and still-live matches, and tables it does not know. A series ref
// accepts any match number k (one table plays a whole best-of-three); a
// Bo1 ref accepts only k == 1. It never calls back into the Registry and
// never takes a run mutex on the calling goroutine — the match goroutine
// that fires it holds nothing this run may wait on — so the handling moves
// onto the Host's wait group.
func (h *Host) OnMatchEnd(t host.TableID, k int, m protocol.MatchInfo) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	ref, ok := h.tables[t]
	if !ok || m.State == protocol.MatchAborted || m.State == protocol.MatchLive {
		return nil
	}
	if !ref.series && k != 1 {
		return nil
	}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		r := ref.run
		r.mu.Lock()
		r.gameOver(ref, k, m)
		r.mu.Unlock()
	}()
	return nil
}

// gameOver records one finished match and drives the run forward. The
// caller holds r.mu. A callback whose attempt is no longer the series'
// in-flight game — a duplicate or stale callback for a table the log has
// already moved past — is dropped. A Bo1 match is buffered for flush's
// pairing-order log slot; a series match is logged at once, because
// its series' own entries are a pure function of the seed and the game
// outcomes.
func (r *run) gameOver(ref gameRef, k int, m protocol.MatchInfo) {
	if ref.series {
		r.seriesGameOver(ref, k, m)
		return
	}
	live := false
	for _, att := range r.ev.InFlight() {
		if att.Table == m.Table {
			live = true
			break
		}
	}
	if !live {
		return
	}
	r.outcomes[[2]int{ref.a, ref.b}] = outcome{ref: ref, m: m}
	r.h.mu.Lock()
	delete(r.h.tables, host.TableID(m.Table))
	r.h.mu.Unlock()
	r.advance()
	r.broadcast()
}

// seriesGameOver logs one finished match of a best-of-three series ref and
// drives the run forward: no buffering, the series' entries log in game
// order. A stale callback — no in-flight attempt on this table playing
// game firstGame+k-1 — is dropped. A finished match logs GameEnded and
// keeps the table's ref (the series continues on it); a crashed one aborts
// the attempt, drops the table, and past the attempt cap logs the game as
// drawn so the series goes on. The caller holds r.mu.
func (r *run) seriesGameOver(ref gameRef, k int, m protocol.MatchInfo) {
	game := ref.firstGame + k - 1
	var att *event.GameAttempt
	inflight := r.ev.InFlight()
	for i := range inflight {
		if inflight[i].Table == m.Table && inflight[i].Game == game {
			att = &inflight[i]
			break
		}
	}
	if att == nil {
		return
	}
	if m.State == protocol.MatchFinished {
		var winner *int
		if m.Winner != nil {
			w := ref.a
			if *m.Winner == 1 {
				w = ref.b
			}
			winner = &w
		}
		if err := r.apply(event.Entry{
			Kind: event.KindGameEnded, Round: ref.round, A: ref.a, B: ref.b,
			Game: game, Attempt: att.Attempt, Winner: winner, Head: m.Head,
		}); err != nil {
			return
		}
		r.advance()
		r.broadcast()
		return
	}
	// MatchCrashed: abort the attempt, drop the table, and past the cap
	// log the game as a draw so the series continues.
	if err := r.apply(event.Entry{
		Kind: event.KindGameAborted, Round: ref.round, A: ref.a, B: ref.b,
		Game: game, Attempt: att.Attempt, Reason: "crashed",
	}); err != nil {
		return
	}
	r.h.mu.Lock()
	delete(r.h.tables, host.TableID(m.Table))
	r.h.mu.Unlock()
	delete(r.seriesTable, [2]int{ref.a, ref.b})
	if att.Attempt >= r.maxAttempts() {
		head := m.Head
		if head == "" {
			head = "crashed"
		}
		if err := r.apply(event.Entry{
			Kind: event.KindGameEnded, Round: ref.round, A: ref.a, B: ref.b,
			Game: game, Attempt: att.Attempt, Head: head,
		}); err != nil {
			return
		}
	}
	r.advance()
	r.broadcast()
}

// flush applies every buffered result that can be logged now, in pairing
// order: only the first pending series, and only once its outcome is
// buffered. The caller holds r.mu.
func (r *run) flush() bool {
	applied := false
	for {
		ps := r.ev.PendingSeries()
		if len(ps) == 0 {
			return applied
		}
		sp := ps[0]
		key := [2]int{sp.A, sp.B}
		o, ok := r.outcomes[key]
		if !ok {
			return applied
		}
		delete(r.outcomes, key)
		if err := r.applyOutcome(sp, o); err != nil {
			return applied
		}
		applied = true
	}
}

// applyOutcome logs one buffered result. A finished match logs GameEnded
// with its winner and head, then closes the series; a crashed match aborts
// the attempt, and once no attempts are left closes the series as a draw.
// The caller holds r.mu.
func (r *run) applyOutcome(sp event.SeriesSpec, o outcome) error {
	a, b := sp.A, sp.B
	switch o.m.State {
	case protocol.MatchFinished:
		var winner *int
		score := [2]int{0, 0}
		if o.m.Winner != nil {
			w := a
			if *o.m.Winner == 1 {
				w = b
			}
			winner = &w
			if w == a {
				score[0] = 1
			} else {
				score[1] = 1
			}
		}
		if err := r.apply(event.Entry{
			Kind: event.KindGameEnded, Round: sp.Round, A: a, B: b,
			Game: o.ref.game, Attempt: o.ref.attempt, Winner: winner, Head: o.m.Head,
		}); err != nil {
			return err
		}
		return r.apply(event.Entry{
			Kind: event.KindSeriesEnded, Round: sp.Round, A: a, B: b, Score: score,
		})
	case protocol.MatchCrashed:
		if err := r.apply(event.Entry{
			Kind: event.KindGameAborted, Round: sp.Round, A: a, B: b,
			Game: o.ref.game, Attempt: o.ref.attempt, Reason: "crashed",
		}); err != nil {
			return err
		}
		if o.ref.attempt < r.maxAttempts() {
			return nil
		}
		head := o.m.Head
		if head == "" {
			head = "crashed"
		}
		if err := r.apply(event.Entry{
			Kind: event.KindGameEnded, Round: sp.Round, A: a, B: b,
			Game: o.ref.game, Attempt: o.ref.attempt, Head: head,
		}); err != nil {
			return err
		}
		return r.apply(event.Entry{
			Kind: event.KindSeriesEnded, Round: sp.Round, A: a, B: b, Score: [2]int{0, 0},
		})
	}
	return nil
}

// maxAttempts is the series-level cap on game attempts: 0 means 3.
func (r *run) maxAttempts() int {
	if r.h.opts.MaxAttempts > 0 {
		return r.h.opts.MaxAttempts
	}
	return 3
}
