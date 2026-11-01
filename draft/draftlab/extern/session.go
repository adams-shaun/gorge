package extern

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

// waiter is one call's delivery slot. The session sends the answer (or,
// on death or close, an error) on ch; the caller also holds ctx for its
// own deadline. A cancelled flag marks a call whose ctx ended before its
// answer arrived: the answer is discarded, but the hub count is still
// released exactly once.
type waiter struct {
	ch        chan callResult
	cancelled atomic.Bool
}

// callResult is one delivery: the agent's answer, or the error that ended
// the request (exactly one is set).
type callResult struct {
	ans wire.Answer
	err error
}

// reqRec is one request the session tracks: from enqueue until its answer
// (or its caller's ctx, or session death) it sits in the queue or the
// in-flight batch, carrying its hub count.
type reqRec struct {
	w  *waiter
	it wire.Item
	// g is the hub gameRec the request was counted on (nil = unscoped).
	g        *gameRec
	released atomic.Bool
}

// session is one Agent's connection: one writer goroutine draining an
// ordered outgoing queue (batches, stream_end, bye), one reader goroutine
// reading result/error, at most one batch in flight, batch numbers from 1
// and item ids unique per session.
//
// Lock order is hub.mu BEFORE session.mu, always; session code never calls
// into the hub while holding session.mu. The condition variable wakes the
// writer on every enqueue, death and close.
type session struct {
	agent *Agent

	wr   *wire.Writer
	rd   *wire.Reader
	tr   transport
	conn io.ReadWriteCloser

	mu   sync.Mutex
	cond *sync.Cond

	// outgoing, guarded by mu
	outQ       []wire.Message
	byePending bool // writer closes the write half after draining outQ
	closing    bool // Close enqueued a bye; no further flushes or sends

	// requests, guarded by mu
	queue     []*reqRec          // not yet sent
	pending   map[uint64]*reqRec // the in-flight batch's items
	pendingNo uint64             // the in-flight batch number (0 = none)
	nextID    uint64             // item ids, unique per session
	batchNo   uint64             // batch numbers, from 1

	// death, guarded by mu except the flag itself
	deadFlag atomic.Bool
	deadErr  error

	doneCh       chan struct{} // closed when the reader goroutine exits
	doneOnce     sync.Once
	writerDoneCh chan struct{} // closed when the writer goroutine exits
}

// transport holds the per-transport half-close hooks. writeCloser closes
// the write half after bye (nil when the conn cannot half-close); closer
// unblocks a blocked reader by closing the read side; finish runs after
// the reader exits (exec's cmd.Wait).
type transport struct {
	writeCloser io.Closer
	closer      io.Closer
	finish      func() error
}

// gone builds a session-death error: always wrapping ErrAgentGone, and
// also wrapping ErrProtocol when the cause is a wire violation. The
// ErrProtocol wrap is added at most once: a violation that already carries
// it (a wire.Validate error) is wrapped directly, so the chain never
// repeats the sentinel.
func gone(cause string, violation error) error {
	if violation == nil {
		return fmt.Errorf("%w: %s", ErrAgentGone, cause)
	}
	if !errors.Is(violation, ErrProtocol) {
		violation = fmt.Errorf("%w: %v", ErrProtocol, violation)
	}
	return fmt.Errorf("%w: %s: %w", ErrAgentGone, cause, violation)
}

// call sends one item and waits for its answer. game is the caller's game
// key; nil, or a key never begun, makes the request unscoped (counted per
// request). The item's id is assigned here and any id the caller set is
// ignored. A ctx that ends first returns ctx.Err() at once; an item still
// queued is dropped (and its hub count released), one already in its batch
// stays there and a late answer is discarded.
func (a *Agent) call(ctx context.Context, game any, it wire.Item) (wire.Answer, error) {
	s := a.sess
	if s.deadFlag.Load() || s.isClosing() {
		return wire.Answer{}, s.stoppedError()
	}
	w := &waiter{ch: make(chan callResult, 1)}
	it.ID = 0
	if err := a.hub.submit(s, game, it, w); err != nil {
		return wire.Answer{}, err
	}
	select {
	case r := <-w.ch:
		return r.ans, r.err
	case <-ctx.Done():
		s.cancelRequest(w)
		return wire.Answer{}, ctx.Err()
	}
}

// stoppedError is the at-once failure of a call on a dead or closing
// session.
func (s *session) stoppedError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deadErr != nil {
		return s.deadErr
	}
	return fmt.Errorf("%w: session closed", ErrAgentGone)
}

func (s *session) isClosing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closing
}

// beginGame returns the game key's process-wide number (assigned at its
// first begin). Streams are named "g<n>.<seat>" with it.
func (a *Agent) beginGame(game any) uint64 { return a.hub.beginGame(game) }

// endGame drops the game key's last reference; when the count reaches zero
// the key leaves the hub's live set.
func (a *Agent) endGame(game any) { a.hub.endGame(game) }

// sendStreamEnd tells the agent that one seat's game ended (its
// per-stream state may be dropped). It is out of band, ordered after any
// batch already queued.
func (a *Agent) sendStreamEnd(stream string) {
	s := a.sess
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deadFlag.Load() || s.closing {
		return
	}
	s.outQ = append(s.outQ, wire.Message{V: wire.Version, Type: wire.TStreamEnd, Stream: stream})
	s.cond.Broadcast()
}

// enqueueLocked records one request on the session queue with its
// session-unique id and the matching stats counter. Caller holds s.mu.
func (s *session) enqueueLocked(rec *reqRec) {
	s.nextID++
	rec.it.ID = s.nextID
	s.queue = append(s.queue, rec)
	switch rec.it.Kind {
	case wire.KindPick:
		s.agent.statsPicks.Add(1)
	case wire.KindBuild:
		s.agent.statsBuilds.Add(1)
	default:
		s.agent.statsDecides.Add(1)
	}
}

// flushLocked composes and queues one batch from the queue's head when the
// flush condition holds. Caller holds hub.mu (for live/waiting) and it
// takes s.mu itself.
func (s *session) flushLocked(live, waiting int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deadFlag.Load() || s.closing || s.pendingNo != 0 || len(s.queue) == 0 {
		return
	}
	mb := s.agent.maxBatch
	if s.agent.opts.Flush != FlushReady && waiting != live && len(s.queue) < mb {
		return
	}
	n := min(len(s.queue), mb)
	msg := wire.Message{V: wire.Version, Type: wire.TBatch}
	for i := 0; i < n; i++ {
		rec := s.queue[i]
		s.pending[rec.it.ID] = rec
		msg.Items = append(msg.Items, rec.it)
	}
	s.queue = s.queue[n:]
	s.batchNo++
	s.pendingNo = s.batchNo
	msg.Batch = s.batchNo
	s.outQ = append(s.outQ, msg)
	s.cond.Broadcast()
	s.agent.statsBatches.Add(1)
	if b := int64(n); b > s.agent.statsMaxBatch.Load() {
		s.agent.statsMaxBatch.Store(b)
	}
}

// cancelRequest drops a still-queued request whose caller's ctx ended (and
// releases its hub count). A request already in its batch stays; its late
// answer is discarded at delivery.
func (s *session) cancelRequest(w *waiter) {
	w.cancelled.Store(true)
	s.mu.Lock()
	var rel []*reqRec
	for i, rec := range s.queue {
		if rec.w == w {
			rel = append(rel, rec)
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
	if len(rel) > 0 {
		s.agent.hub.release(rel...)
	}
}

// writeLoop drains the ordered outgoing queue. After the queue drains past
// a pending bye it closes the write half and exits; on death it exits at
// once; a write error kills the session.
func (s *session) writeLoop() {
	defer close(s.writerDoneCh)
	for {
		s.mu.Lock()
		for len(s.outQ) == 0 && !s.byePending && !s.deadFlag.Load() {
			s.cond.Wait()
		}
		if s.deadFlag.Load() {
			s.mu.Unlock()
			return
		}
		if len(s.outQ) == 0 {
			// byePending: everything went out; end gorge's side.
			s.mu.Unlock()
			if wc := s.tr.writeCloser; wc != nil {
				_ = wc.Close()
			}
			return
		}
		m := s.outQ[0]
		s.outQ = s.outQ[1:]
		s.mu.Unlock()
		if err := s.wr.Write(m); err != nil {
			s.kill(gone(fmt.Sprintf("write: %v", err), nil))
			return
		}
	}
}

// readLoop reads result/error until the stream ends. Every shape violation,
// an agent error message, EOF (while not closing), a read error or a line
// over the cap kills the session; EOF after a bye is the clean close.
func (s *session) readLoop() {
	for {
		m, err := s.rd.Read()
		if err != nil {
			switch {
			case errors.Is(err, io.EOF) && s.isClosing():
				// clean close: the agent saw the bye and left
			case errors.Is(err, io.EOF):
				s.kill(gone("agent closed the connection", nil))
			case errors.Is(err, wire.ErrLineTooLong):
				s.kill(gone(err.Error(), err))
			default:
				s.kill(gone(fmt.Sprintf("read: %v", err), nil))
			}
			s.finishRead()
			return
		}
		if err := s.handle(m); err != nil {
			s.kill(err)
			s.finishRead()
			return
		}
	}
}

// finishRead runs once when the reader goroutine exits: it fails whatever
// requests the session still tracks and releases the hub counts, then
// closes doneCh (Close waits on it).
func (s *session) finishRead() {
	s.failRemaining()
	s.doneOnce.Do(func() { close(s.doneCh) })
}

// failRemaining errors and releases every request still queued or in
// flight, and fails its callers.
func (s *session) failRemaining() {
	s.mu.Lock()
	err := s.deathErrorLocked()
	recs := s.queue
	s.queue = nil
	for id, r := range s.pending {
		recs = append(recs, r)
		delete(s.pending, id)
	}
	s.pendingNo = 0
	s.cond.Broadcast()
	s.mu.Unlock()
	// kill logged the death already; a clean end after bye logs its own
	// distinct line instead of repeating a death-shaped error.
	if s.agent.opts.Log != nil && !s.deadFlag.Load() {
		s.agent.opts.Log("extern: session ended")
	}
	s.failWaiters(recs, err)
	s.agent.hub.release(recs...)
}

// failWaiters delivers err to every not-yet-cancelled caller (channel
// buffered, so this cannot block).
func (s *session) failWaiters(recs []*reqRec, err error) {
	for _, rec := range recs {
		if !rec.w.cancelled.Load() {
			select {
			case rec.w.ch <- callResult{err: err}:
			default:
			}
		}
	}
}

// handle validates and dispatches one agent message.
func (s *session) handle(m wire.Message) error {
	if err := m.Validate(wire.FromAgent); err != nil {
		return gone(err.Error(), err)
	}
	switch m.Type {
	case wire.TResult:
		return s.processResult(m)
	case wire.TError:
		return gone(fmt.Sprintf("agent sent error %q: %s", m.Code, m.Message), nil)
	default:
		return gone(fmt.Sprintf("field %q: unexpected %q from the agent", "type", m.Type),
			fmt.Errorf("unexpected %q from the agent", m.Type))
	}
}

// processResult validates a result against the one batch in flight and
// delivers each answer to its caller. The batch number must be the one in
// flight; the answer ids must be exactly that batch's ids, each once; and
// each answer's one field must fit its item's kind (an error answer fits
// any kind). Any violation kills the session.
func (s *session) processResult(m wire.Message) error {
	s.mu.Lock()
	var done []*reqRec
	var vErr error
	if s.deadFlag.Load() {
		s.mu.Unlock()
		return nil // already dying; discard
	}
	if s.pendingNo == 0 || m.Batch != s.pendingNo {
		vErr = fmt.Errorf("field %q: got batch %d, want the one batch in flight (%d)",
			"batch", m.Batch, s.pendingNo)
	} else if len(m.Answers) != len(s.pending) {
		vErr = fmt.Errorf("field %q: %d answers for %d items", "answers", len(m.Answers), len(s.pending))
	} else {
		for _, a := range m.Answers {
			rec, ok := s.pending[a.ID]
			if !ok {
				vErr = fmt.Errorf("answers: field %q: %d is not an item of batch %d", "id", a.ID, m.Batch)
				break
			}
			if !answerFits(rec.it.Kind, a) {
				vErr = fmt.Errorf("answer for item %d (%s): its field does not fit a %s item", a.ID, rec.it.Kind, rec.it.Kind)
				break
			}
		}
	}
	if vErr != nil {
		s.mu.Unlock()
		return gone(vErr.Error(), vErr)
	}
	for _, a := range m.Answers {
		rec := s.pending[a.ID]
		delete(s.pending, a.ID)
		if !rec.w.cancelled.Load() {
			select {
			case rec.w.ch <- callResult{ans: a}:
			default:
			}
		}
		done = append(done, rec)
	}
	if len(s.pending) == 0 {
		s.pendingNo = 0
	}
	s.mu.Unlock()
	s.agent.hub.release(done...)
	return nil
}

// answerFits reports whether the answer's one set field fits the item's
// kind; an error answer fits any kind. Validate already ensured exactly
// one field is set.
func answerFits(kind string, a wire.Answer) bool {
	if a.Error != nil {
		return true
	}
	switch kind {
	case wire.KindPick:
		return a.Pick != ""
	case wire.KindBuild:
		return a.Deck != nil
	case wire.KindDecide:
		return a.Intent != nil
	}
	return false
}

// kill ends the session once: it fails every tracked request with err,
// releases their hub counts, logs one line, wakes the writer and closes
// both the write side (so a child blocked reading stdin sees EOF and is
// reaped) and the read side (so a blocked reader exits). Later kills are
// no-ops.
func (s *session) kill(err error) {
	s.mu.Lock()
	if !s.deadFlag.CompareAndSwap(false, true) {
		s.mu.Unlock()
		return
	}
	s.deadErr = err
	recs := s.queue
	s.queue = nil
	for id, r := range s.pending {
		recs = append(recs, r)
		delete(s.pending, id)
	}
	s.pendingNo = 0
	s.cond.Broadcast()
	s.mu.Unlock()
	if s.agent.opts.Log != nil {
		s.agent.opts.Log("extern: session died: " + err.Error())
	}
	s.failWaiters(recs, err)
	s.agent.hub.release(recs...)
	if s.tr.writeCloser != nil {
		_ = s.tr.writeCloser.Close()
	}
	if s.tr.closer != nil {
		_ = s.tr.closer.Close()
	}
}

// deathErrorLocked is the recorded death error, or a generic one. Caller
// holds s.mu.
func (s *session) deathErrorLocked() error {
	if s.deadErr != nil {
		return s.deadErr
	}
	return fmt.Errorf("%w: session closed", ErrAgentGone)
}
