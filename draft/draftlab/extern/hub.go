package extern

import (
	"sync"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

// The hub is the process-wide batching coordinator (E5 plan §9, plan §6).
// It decides when each session flushes its outgoing queue.
//
// Why process-wide, not per agent: with one hub per agent, a game whose
// seats sit on agents A and B is "live but not waiting" for A while it
// waits on B, and B's barrier can then wait on a game that waits on A — a
// deadlock. Counting "waiting on ANY agent" under one lock removes it
// (TestTwoAgentsNoDeadlock pins it).
//
// Liveness argument (plan §6.1): between beginGame and endGame a lab game
// is either computing (the engine or a built-in seat — finite work) or
// blocked on the hub, because a caller with nothing queued has either
// answered or is about to enqueue. So when nothing can make progress,
// every live game has a request queued or in flight, waiting == live, and
// the barrier fires some batch. The re-evaluation points cover every
// transition: each enqueue (a call), each answered batch, each endGame and
// each session death — one of them always fires the moment the condition
// turns true.
//
// Flush rules. A session with a non-empty queue and no batch in flight
// sends min(len(queue), maxBatch) items FIFO when:
//   - FlushReady: always (flush whatever is queued whenever idle); or
//   - FlushBarrier (the default): waiting == live — every live game is
//     blocked on an external agent — or its queue already holds maxBatch
//     items (an override so one busy game cannot starve forever).
//
// A call whose game key was never begun is treated as unscoped (counted
// per request, always waiting), so a stray call can never stall a barrier.
//
// The hub is shared by every Agent in the process (procHub); package tests
// may dial into a private hub instead. It never imports time: flushing is
// driven by these transitions only, never by a timer.
type hub struct {
	mu        sync.Mutex
	games     map[any]*gameRec
	unscoped  int        // unscoped requests queued or in flight (each its own one-request game)
	nextNum   uint64     // process-wide game-number counter, from 1, never reused
	sessions  []*session // registration order (Dial order)
	onEnqueue func()     // test hook: fired after a request is enqueued; nil in production
}

// gameRec is one begun game key: its refcount, its process-wide number and
// how many of its requests are queued or in flight across all sessions.
type gameRec struct {
	refs        int
	num         uint64
	outstanding int
}

// newHub builds a private hub (tests).
func newHub() *hub {
	return &hub{games: map[any]*gameRec{}}
}

// procHub is the process-wide hub every Dial/DialConn agent joins.
var procHub = newHub()

// register adds a session to the hub's flush walk, in registration order.
func (h *hub) register(s *session) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions = append(h.sessions, s)
}

// beginGame takes one reference on the game key and returns the key's
// process-wide number, assigned at the key's first begin and never reused.
func (h *hub) beginGame(game any) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	g := h.games[game]
	if g == nil {
		h.nextNum++
		g = &gameRec{num: h.nextNum}
		h.games[game] = g
	}
	g.refs++
	return g.num
}

// endGame drops one reference; the key (and its counts) is removed at
// zero. A request still outstanding for a removed key releases harmlessly.
func (h *hub) endGame(game any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if g := h.games[game]; g != nil {
		g.refs--
		if g.refs <= 0 {
			delete(h.games, game)
		}
	}
	h.maybeFlushLocked()
}

// submit counts one request (on its begun game key, or unscoped) and
// enqueues it on the session, then re-evaluates the flush condition. It
// fails at once when the session is already dead: no counting, no
// enqueueing, the caller gets the death error.
func (h *hub) submit(s *session, game any, it wire.Item, w *waiter) error {
	h.mu.Lock()
	s.mu.Lock()
	if s.deadFlag.Load() {
		err := s.deathErrorLocked()
		s.mu.Unlock()
		h.mu.Unlock()
		return err
	}
	rec := &reqRec{w: w, it: it}
	if g := h.games[game]; game != nil && g != nil {
		g.outstanding++
		rec.g = g
	} else {
		h.unscoped++
	}
	s.enqueueLocked(rec)
	s.mu.Unlock()
	h.maybeFlushLocked()
	h.mu.Unlock()
	if h.onEnqueue != nil {
		h.onEnqueue()
	}
	return nil
}

// release returns recs' hub counts (each exactly once) and re-evaluates
// the flush condition. A record whose game key was since removed drops
// silently.
func (h *hub) release(recs ...*reqRec) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range recs {
		if !r.released.CompareAndSwap(false, true) {
			continue
		}
		if r.g != nil {
			if r.g.outstanding > 0 {
				r.g.outstanding--
			}
		} else if h.unscoped > 0 {
			h.unscoped--
		}
	}
	h.maybeFlushLocked()
}

// maybeFlushLocked walks the sessions in registration order and flushes
// each eligible one. Caller holds h.mu.
func (h *hub) maybeFlushLocked() {
	live := len(h.games) + h.unscoped
	waiting := h.unscoped
	for _, g := range h.games {
		if g.outstanding > 0 {
			waiting++
		}
	}
	for _, s := range h.sessions {
		s.flushLocked(live, waiting)
	}
}
