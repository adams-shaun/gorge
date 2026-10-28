package sbv2http

import (
	"context"
	"errors"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/spellbench/humanseat"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// seatKey names one seat of one match of one table, the identity every route
// and every tap answer is scoped to.
type seatKey struct {
	table host.TableID
	match int
	seat  state.PlayerID
}

// errSuperseded is returned to a Choose whose request was replaced by a newer
// park. The codec treats a Choose error as "give v2 up for this decision", and
// the decision stays pending natively, so the client can answer it on the
// native wire.
var errSuperseded = errors.New("sbv2: request superseded")

// request is one open v2 choose question. Its id is stable for the life of the
// question and is what POST .../send names; the first answer wins, and every
// later one is stale.
type request struct {
	id     uint64
	ask    humanseat.Ask
	result chan int
	done   chan struct{}
	once   sync.Once

	// nativeOnly marks the synthetic request published when the codec leaves a
	// decision to the native wire. It carries the extensions and the native
	// view, but no candidates: POST .../send is stale for it.
	nativeOnly bool

	// closed and answered are read and written under seatState.mu.
	closed   bool
	answered bool
}

func (r *request) finish() {
	r.once.Do(func() { close(r.done) })
}

// seatState is the far end of one human seat's v2 codec: it receives each
// Choose from the humanseat.Seat on the codec's own goroutine, publishes it to
// the HTTP fronts, and blocks until an answer arrives or the decision is
// superseded.
type seatState struct {
	mu      sync.Mutex
	next    uint64
	current *request
	wake    chan struct{}
	// nativeOnly counts decisions the codec left to the native wire. It is
	// the signal a client (and this package's tests) uses to stop waiting
	// for a v2 request and answer natively instead.
	nativeOnly int
	// ext builds the opt-in extensions for this seat. Nil when the hub was not
	// asked for extensions (the native wire).
	ext *extensionBuilder
}

func newSeatState() *seatState {
	return &seatState{wake: make(chan struct{})}
}

func (s *seatState) setExtension(ext *extensionBuilder) {
	s.mu.Lock()
	s.ext = ext
	s.mu.Unlock()
}

// notifyLocked wakes every stream watching this seat. Called under s.mu.
func (s *seatState) notifyLocked() {
	close(s.wake)
	s.wake = make(chan struct{})
}

// snapshot returns the open request and the channel that closes on the next
// change, both under one lock so a change between the read and the wait is
// never missed.
func (s *seatState) snapshot() (*request, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current, s.wake
}

// Choose implements humanseat.Answerer. The returned index is the candidate id
// the v2 client picked.
func (s *seatState) Choose(ctx context.Context, ask humanseat.Ask) (int, error) {
	s.mu.Lock()
	s.next++
	req := &request{id: s.next, ask: ask, result: make(chan int, 1), done: make(chan struct{})}
	prev := s.current
	s.current = req
	s.notifyLocked()
	s.mu.Unlock()

	if prev != nil {
		// The newer park supersedes the previous question even if its own
		// context has not fired yet. Its Choose returns errSuperseded so the
		// codec drops it and leaves the native decision pending.
		prev.muClose()
	}

	select {
	case i := <-req.result:
		s.finish(req)
		return i, nil
	case <-ctx.Done():
		s.finish(req)
		return 0, ctx.Err()
	case <-req.done:
		s.finish(req)
		return 0, errSuperseded
	}
}

// muClose marks a superseded request closed and wakes its Choose. It takes the
// seat lock so a concurrent answer sees the closed flag atomically with the
// request still being current.
func (r *request) muClose() {
	r.once.Do(func() { close(r.done) })
}

func (s *seatState) finish(req *request) {
	s.mu.Lock()
	req.closed = true
	if s.current == req {
		s.current = nil
		s.notifyLocked()
	}
	s.mu.Unlock()
	req.finish()
}

// NativeOnly implements humanseat.Answerer. It publishes a candidate-less
// request whose x_gorge_human_v1 says native_only and whose x_gorge_view_v1
// carries the native view, so the client can render the moment and answer it
// on the native wire; the next v2 decision supersedes it.
func (s *seatState) NativeOnly(n humanseat.NativeOnly) {
	s.mu.Lock()
	s.nativeOnly++
	s.next++
	var extensions protocol.ExtensionMap
	if s.ext != nil {
		if built, err := s.ext.NativeExtension(n); err == nil {
			extensions = built
		}
	}
	req := &request{
		id:         s.next,
		nativeOnly: true,
		ask:        humanseat.Ask{GameID: n.GameID, Decision: protocol.SeatDecision{Extensions: extensions}},
		done:       make(chan struct{}),
	}
	prev := s.current
	s.current = req
	s.notifyLocked()
	s.mu.Unlock()
	if prev != nil {
		prev.muClose()
	}
}

// nativeOnlyCount reports how many decisions the codec left to the native
// wire. Tests use it to decide when a native answer is expected; nothing in
// the production routes reads it (a null frame is the wire signal).
func (s *seatState) nativeOnlyCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nativeOnly
}

// open reports whether requestID names the seat's open, answerable v2
// request. A synthetic native-only request is never answerable through send.
func (s *seatState) open(requestID uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	req := s.current
	return req != nil && req.id == requestID && !req.closed && !req.answered && !req.nativeOnly
}

// answer delivers candidate id as the answer to request requestID. It reports
// false when no request with that id is open — already answered, superseded,
// or never this seat's.
func (s *seatState) answer(requestID uint64, candidateID int) bool {
	s.mu.Lock()
	req := s.current
	if req == nil || req.id != requestID || req.closed || req.answered {
		s.mu.Unlock()
		return false
	}
	if req.nativeOnly || req.ask.Pose == nil || candidateID < 0 || candidateID >= len(req.ask.Pose.Candidates) {
		s.mu.Unlock()
		return false
	}
	req.answered = true
	s.mu.Unlock()

	select {
	case req.result <- candidateID:
		return true
	default:
		return false
	}
}

// Hub is the per-seat request state shared by the tap factory and the HTTP
// handler. It is process-local and grows only with real (table, match, seat)
// triples, which are bounded like ManaBrew's connection map.
type Hub struct {
	mu    sync.Mutex
	seats map[seatKey]*seatState
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{seats: map[seatKey]*seatState{}}
}

func (h *Hub) seat(key seatKey) *seatState {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.seats[key]
	if !ok {
		s = newSeatState()
		h.seats[key] = s
	}
	return s
}

// Tap is the host.HumanTapFactory gorged installs on the registry: for a
// two-seat, constructed, AutoMana table it returns the v2 codec wired to this
// hub; every other table gets nil, so no tap is installed and the seat is an
// ordinary native human seat. The host's attach path already refuses a tapped
// manual-mana table, which is why the AutoMana check is part of "covers".
func (h *Hub) Tap(table host.TableID, match int, autoPayMana bool, mc *bots.MatchContext) (host.HumanTap, error) {
	if mc == nil {
		return nil, errors.New("sbv2: nil match context")
	}
	if !autoPayMana || mc.Seats != 2 || mc.Format != "constructed" {
		return nil, nil
	}
	s := h.seat(seatKey{table: table, match: match, seat: state.PlayerID(mc.Seat)})
	ext := newExtensionBuilder()
	s.setExtension(ext)
	return humanseat.New(humanseat.Config{Seats: mc.Seats, Format: mc.Format, Answerer: s, Extend: ext.Extend})
}
