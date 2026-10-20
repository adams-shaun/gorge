package host

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"github.com/adams-shaun/gorge/server/protocol"
)

// Session is one client connection's worth of subscriptions: a bounded
// outbound channel, a ring of the frames it has sent (for Last-Event-ID
// resume), and the latest widget per table (PL-5: widgets are coalesced,
// never ring-buffered, never given an id).
type Session struct {
	ID string
	// serial is an internal creation sequence for deterministic fan-out. It
	// is separate from ID because IDs are unguessable random capabilities.
	serial uint64

	mu      sync.Mutex
	out     chan protocol.Frame
	ring    []protocol.Frame // oldest first, len <= cap(out); Options.Ring sizes both
	nextID  uint64
	subs    map[TableID]string // table -> mode; TableAll for "every table, overview"
	widgets map[TableID]protocol.Frame
	// seatTable is the table this session proved a seat claim for (the
	// HTTP layer binds it from the stream's token). It gates private
	// on-demand tables: a wildcard overview and an explicit subscription
	// both see one only when the claim names it.
	seatTable  TableID
	dropped    int
	overflowed bool
	closed     bool
}

// TableAll as a TableID, for the subscription map.
const TableAll TableID = protocol.TableAll

// OpenSession registers a new session with an unguessable identifier issued
// only in its hello frame. A seat-aware subscription can bind private frames
// to this session, so a sequential id would let another seat claim it before
// its owner subscribes. serial preserves deterministic fan-out order without
// making the public identifier predictable.
func (r *Registry) OpenSession() *Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	var id string
	for {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			panic("host: minting session id: " + err.Error())
		}
		id = "s" + hex.EncodeToString(b[:])
		if _, exists := r.sessions[id]; !exists {
			break
		}
	}
	r.nextSess++
	s := &Session{ID: id, serial: uint64(r.nextSess), out: make(chan protocol.Frame, r.opts.Ring),
		subs: map[TableID]string{}, widgets: map[TableID]protocol.Frame{}}
	r.sessions[s.ID] = s
	return s
}

// Session looks a session up by id.
func (r *Registry) Session(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	return s, ok
}

// CloseSession drops the session's subscriptions and closes Out.
func (r *Registry) CloseSession(id string) {
	r.mu.Lock()
	s, ok := r.sessions[id]
	delete(r.sessions, id)
	r.mu.Unlock()
	if ok {
		s.close()
	}
}

// closeSessions closes every session, e.g. once Registry.Close has stopped
// every table.
func (r *Registry) closeSessions() {
	r.mu.Lock()
	ss := r.sessions
	r.sessions = map[string]*Session{}
	r.mu.Unlock()
	for _, s := range ss {
		s.close()
	}
}

func (s *Session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.out)
	}
}

// Out is the resumable frame stream. It is closed after an overflow (the
// http layer then sends the overflow frame itself) or CloseSession.
func (s *Session) Out() <-chan protocol.Frame { return s.out }

// Overflowed reports whether the channel ever filled and how many frames
// were dropped before Out was closed.
func (s *Session) Overflowed() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped, s.overflowed
}

// push assigns the next id, records the frame in the ring and hands it to
// the channel without ever blocking: a full channel means the reader is
// too slow, so the session overflows and is closed (the engine loop must
// never wait on a client). Returns false once the session is closed or
// overflowed; every push attempted after the overflow still counts toward
// dropped, so Overflowed's count is the true number of frames the client
// never received, not just the one that tipped the channel over.
func (s *Session) push(f protocol.Frame) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overflowed {
		s.dropped++
		return false
	}
	if s.closed {
		return false
	}
	s.nextID++
	f.ID = s.nextID
	if len(s.ring) == cap(s.out) {
		copy(s.ring, s.ring[1:])
		s.ring = s.ring[:len(s.ring)-1]
	}
	s.ring = append(s.ring, f)
	select {
	case s.out <- f:
		return true
	default:
		s.dropped++
		s.overflowed = true
		s.closed = true
		close(s.out)
		return false
	}
}

// setWidget replaces the latest widget for a table. Called with no frame
// id (PL-5): widgets bypass the ring entirely.
func (s *Session) setWidget(id TableID, f protocol.Frame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.widgets[id] = f
}

// TakeWidgets returns and clears the latest widget per table, in table-id
// order. The SSE writer calls it on every tick.
func (s *Session) TakeWidgets() []protocol.Frame {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.widgets))
	for id := range s.widgets {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]protocol.Frame, 0, len(ids))
	for _, id := range ids {
		out = append(out, s.widgets[TableID(id)])
	}
	s.widgets = map[TableID]protocol.Frame{}
	return out
}

// Since returns every ring frame with ID > id, in order. ok is false when
// id is older than the oldest frame still in the ring (or the ring is
// empty and id is not the current head), in which case the caller must
// start the client over with a fresh hello and snapshots.
func (s *Session) Since(id uint64) ([]protocol.Frame, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == s.nextID {
		return nil, true
	}
	if len(s.ring) == 0 || id < s.ring[0].ID-1 || id > s.nextID {
		return nil, false
	}
	var out []protocol.Frame
	for _, f := range s.ring {
		if f.ID > id {
			out = append(out, f)
		}
	}
	return out, true
}

// BindSeat records the table this session proved a seat claim for. The
// HTTP stream resolves the claim from the request's token; the ManaBrew
// stream resolves it before opening its own session. It never widens a
// session's visibility on its own: Subscribe still requires the claim for a
// private table, and the wildcard overview skips private tables unless the
// claim names them.
func (s *Session) BindSeat(id TableID) {
	s.mu.Lock()
	s.seatTable = id
	s.mu.Unlock()
}

func (s *Session) seatTableFor() TableID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.seatTable
}

// privateTable reports whether a table is a private on-demand play-vs-bot
// table. Event/series tables are on-demand too (seatpolicy.go requires it),
// but they are publicly watchable: the event hub links to them without a seat
// token, so only a play-vs-bot table is fenced behind the session's claim.
func privateTable(cfg TableConfig) bool {
	return cfg.OnDemand && cfg.Event == ""
}

// modeFor is the session's mode for a table: its own entry, else the
// wildcard's. A private play-vs-bot table is visible only to a session whose
// seat claim names it, so the wildcard never announces one to an anonymous
// overview client.
func (s *Session) modeFor(id TableID, private bool) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if private && s.seatTable != id {
		return "", false
	}
	if m, ok := s.subs[id]; ok {
		return m, true
	}
	if m, ok := s.subs[TableAll]; ok {
		return m, true
	}
	return "", false
}

// Hello is the stream's first frame: the session id and the lobby's table
// list. It carries no frame id (the http layer sends it before Last-Event-ID
// resumption applies). Private on-demand play-vs-bot tables are omitted, as
// they are from GET /api/tables: a stream without a claim for one must not
// learn its (sequential) id. A client holding a seat token still receives such
// a table through its explicit focus subscription (Subscribe).
//
// include names tables the caller holds a claim to (the HTTP layer passes a
// resolved seat claim's table). They are appended when they exist and are
// not already in the lobby list, so a seated player keeps the TableInfo the
// client needs for auto-mana and restart without advertising the private table
// to every other stream.
func (r *Registry) Hello(s *Session, include ...TableID) protocol.Frame {
	tables := r.LobbyTables()
	seen := make(map[TableID]bool, len(tables)+len(include))
	for _, ti := range tables {
		seen[TableID(ti.ID)] = true
	}
	for _, id := range include {
		if seen[id] {
			continue
		}
		r.mu.RLock()
		t, ok := r.tables[id]
		r.mu.RUnlock()
		if !ok {
			continue
		}
		tables = append(tables, t.info())
		seen[id] = true
	}
	f, err := protocol.NewFrame(protocol.THello, "", 0, 0, protocol.Hello{Session: s.ID, Tables: tables})
	if err != nil {
		panic("host: hello frame: " + err.Error()) // a marshal failure of our own struct is a bug
	}
	return f
}

// Subscribe adds a table (or every table, overview only) to the session. A
// focus subscription on a live table pushes a snapshot at once so the
// client has a board before the first event frame arrives.
func (r *Registry) Subscribe(s *Session, id TableID, mode string) error {
	if mode != protocol.ModeOverview && mode != protocol.ModeFocus {
		return fmt.Errorf("host: unknown mode %q", mode)
	}
	if id == TableAll {
		if mode != protocol.ModeOverview {
			return fmt.Errorf("host: %q may only be subscribed in overview mode", protocol.TableAll)
		}
		s.mu.Lock()
		s.subs[TableAll] = mode
		s.mu.Unlock()
		return nil
	}
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	t.mu.RLock()
	private := privateTable(t.cfg)
	t.mu.RUnlock()
	// A private table exists only for the client that proved its claim.
	// ErrNotFound keeps the rejection indistinguishable from a missing id,
	// so a guessed sequential gN learns nothing.
	if private && s.seatTableFor() != id {
		return ErrNotFound
	}
	s.mu.Lock()
	s.subs[id] = mode
	s.mu.Unlock()
	if mode == protocol.ModeFocus {
		// fanMu serialises this build-then-push against the match loop's
		// own fan-out (Ruling FL-30): without it, a live burst could land
		// between this snapshot's build and its push, permanently
		// stranding the client behind an already-stale snapshot. push
		// never blocks, so this never parks the match loop on a client.
		t.fanMu.Lock()
		t.mu.RLock()
		m := t.cur
		t.mu.RUnlock()
		if m != nil {
			var f protocol.Frame
			r.projectLive(m, func() { f = r.snapshotFrame(t, m) })
			s.push(f)
		}
		t.fanMu.Unlock()
		r.dropOverflowed([]*Session{s})
	}
	return nil
}

// Unsubscribe removes one table (or the wildcard). Unsubscribing the
// wildcard clears every cached widget, not just a "*" entry (which never
// existed as a widget key), unless a specific per-table subscription still
// justifies keeping them.
func (r *Registry) Unsubscribe(s *Session, id TableID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[id]; !ok {
		return ErrNotFound
	}
	delete(s.subs, id)
	if id == TableAll {
		if len(s.subs) == 0 {
			s.widgets = map[TableID]protocol.Frame{}
		}
	} else {
		delete(s.widgets, id)
	}
	return nil
}

// sessionsFor lists the sessions subscribed to a table with their modes,
// in session-id order (creation order), so fan-out order is deterministic.
func (r *Registry) sessionsFor(id TableID) (ss []*Session, modes []string) {
	r.mu.RLock()
	all := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		all = append(all, s)
	}
	t := r.tables[id]
	r.mu.RUnlock()
	private := false
	if t != nil {
		t.mu.RLock()
		private = privateTable(t.cfg)
		t.mu.RUnlock()
	}
	sort.Slice(all, func(i, j int) bool { return all[i].serial < all[j].serial })
	for _, s := range all {
		if m, ok := s.modeFor(id, private); ok {
			ss = append(ss, s)
			modes = append(modes, m)
		}
	}
	return ss, modes
}

// dropOverflowed unregisters sessions that overflowed during a fan-out.
func (r *Registry) dropOverflowed(ss []*Session) {
	for _, s := range ss {
		if _, of := s.Overflowed(); of {
			r.mu.Lock()
			delete(r.sessions, s.ID)
			r.mu.Unlock()
		}
	}
}
