package host

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/seat"
)

// Options configures a Registry. LoadDeck is required so a caller can never
// forget that the host reads no files for decks itself.
type Options struct {
	Dir      string
	LoadDeck func(name string) (Deck, error)
	// Sleep is the table's only clock read (PL-11): run calls it between
	// matches for Cooldown, and play calls it after every decision for
	// Pace. It must return once d elapses OR stop closes, whichever comes
	// first — that is what lets Close interrupt a table sitting in a long
	// cooldown or a slow pace (Ruling FL-18) instead of blocking up to d.
	// A nil Sleep gets defaultSleep, which does exactly that with a real
	// timer; it is the only place in the package that touches the clock,
	// so a caller wanting a faster-than-realtime test still goes through
	// this field rather than host reading time.Now/time.Sleep itself.
	Sleep      func(d time.Duration, stop <-chan struct{})
	Seats      func(names []string, seed uint64) []seat.Seat
	Sync       bool
	Ring       int
	Cooldown   time.Duration
	MaxIntents int
}

// defaultSleep is installed when Options.Sleep is nil. It is the package's
// only time.Now/time.NewTimer read (PL-11's stated exception): everywhere
// else in host reaches the clock only through the Sleep field.
func defaultSleep(d time.Duration, stop <-chan struct{}) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-stop:
	}
}

// Registry owns the tables and the sessions watching them.
type Registry struct {
	opts Options

	mu       sync.RWMutex
	tables   map[TableID]*table
	sessions map[string]*Session
	nextSess int
	closed   bool
	done     chan struct{} // closed by Close once every table has been told to stop
	wg       sync.WaitGroup
}

// New validates Options and, when Dir is set, reads the registry back from
// disk (Task 12). Nothing starts running until Start/StartAll.
func New(o Options) (*Registry, error) {
	if o.LoadDeck == nil {
		return nil, fmt.Errorf("host: Options.LoadDeck is required")
	}
	if o.Sleep == nil {
		o.Sleep = defaultSleep
	}
	if o.Seats == nil {
		o.Seats = defaultSeats
	}
	if o.Ring == 0 {
		o.Ring = 256
	}
	r := &Registry{opts: o, tables: map[TableID]*table{}, sessions: map[string]*Session{}, done: make(chan struct{})}
	if o.Dir != "" {
		if err := r.load(); err != nil { // Task 12
			return nil, err
		}
	}
	return r, nil
}

// AddTable registers (and persists) a table without starting it.
func (r *Registry) AddTable(c TableConfig) error {
	if err := c.validate(r.opts.LoadDeck); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return fmt.Errorf("host: registry is closed")
	}
	if _, dup := r.tables[c.ID]; dup {
		return fmt.Errorf("host: table %s already exists", c.ID)
	}
	r.tables[c.ID] = newTable(c)
	return r.save() // Task 12; a no-op in memory mode
}

// Start launches the table's goroutine; a second Start is a no-op.
func (r *Registry) Start(id TableID) error {
	r.mu.Lock()
	t, ok := r.tables[id]
	if !ok {
		r.mu.Unlock()
		return ErrNotFound
	}
	if r.closed {
		r.mu.Unlock()
		return fmt.Errorf("host: registry is closed")
	}
	if t.started {
		r.mu.Unlock()
		return nil
	}
	t.started = true
	r.wg.Add(1)
	r.mu.Unlock()
	go r.run(t)
	return nil
}

// StartAll starts every registered table, in ID order.
func (r *Registry) StartAll() error {
	for _, id := range r.ids() {
		if err := r.Start(id); err != nil {
			return err
		}
	}
	return nil
}

// Wait blocks until the table's goroutine has exited: the table is idle,
// halted or the registry was closed. An unknown or never-started table
// returns at once.
func (r *Registry) Wait(id TableID) {
	r.mu.RLock()
	t, ok := r.tables[id]
	started := ok && t.started
	r.mu.RUnlock()
	if !started {
		return
	}
	<-t.done
}

// run is the table's goroutine: match after match while perpetual, until
// a non-perpetual match ends, the registry closes, or a crash halts it.
//
// ctx is derived once, here, for the table's whole lifetime — not once per
// match — and is the sole cancellation path play has into a Seat.Decide
// call that is already blocked when Close runs (Ruling FL-17): t.stop
// itself is only ever polled between decisions, so a seat that does not
// return on its own (a disconnected human, Task 25) would otherwise wedge
// this goroutine, and Close's wg.Wait with it, forever. The bridging
// goroutine below is the only consumer of ctx.Done() other than
// play/Decide; it exits as soon as either side fires, so it never outlives
// run.
func (r *Registry) run(t *table) {
	defer r.wg.Done()
	defer close(t.done)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-t.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	for {
		t.mu.Lock()
		k := t.k + 1
		t.mu.Unlock()
		m, err := r.newMatch(t, k)
		if err != nil {
			r.halt(t, k, err) // Task 13; sets state halted
			return
		}
		t.mu.Lock()
		t.k, t.cur, t.state = k, m, protocol.TableLive
		t.mu.Unlock()
		r.onMatchStart(t, m) // Tasks 10, 12
		final := r.play(ctx, t, m)
		t.mu.Lock()
		t.cur = nil
		t.history = append(t.history, m)
		t.mu.Unlock()
		switch final {
		case protocol.MatchCrashed:
			r.halt(t, k, fmt.Errorf("%s", m.reason))
			return
		case protocol.MatchAborted:
			t.setState(protocol.TableIdle)
			return
		}
		if !t.cfg.Perpetual {
			t.setState(protocol.TableIdle)
			return
		}
		t.setState(protocol.TableCooldown)
		r.opts.Sleep(r.opts.Cooldown, t.stop)
		select {
		case <-t.stop:
			t.setState(protocol.TableIdle)
			return
		default:
		}
	}
}

// halt is D15's second half for the table: it stops and stays stopped,
// recording why. k is the match number the caller was building or had just
// finished — not necessarily t.k, which the newMatch-failure path never
// bumps, so a first-boot halt is addressed to the match that actually
// failed, not match 0. Task 13 adds the crash report file.
func (r *Registry) halt(t *table, k int, err error) {
	reason := err.Error()
	t.mu.Lock()
	t.state = protocol.TableHalted
	t.reason = reason
	t.mu.Unlock()
	r.sendHalted(t, k, reason)
}

// Tables lists every table, sorted by ID.
func (r *Registry) Tables() []protocol.TableInfo {
	out := make([]protocol.TableInfo, 0)
	for _, id := range r.ids() {
		r.mu.RLock()
		t := r.tables[id]
		r.mu.RUnlock()
		out = append(out, t.info())
	}
	return out
}

// Matches lists a table's matches in ascending order; the live one last.
func (r *Registry) Matches(id TableID) ([]protocol.MatchInfo, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	t.mu.RLock()
	ms := append([]*match(nil), t.history...)
	if t.cur != nil {
		ms = append(ms, t.cur)
	}
	t.mu.RUnlock()
	out := make([]protocol.MatchInfo, 0, len(ms))
	for _, m := range ms {
		m.mu.RLock()
		out = append(out, m.info())
		m.mu.RUnlock()
	}
	return out, nil
}

// Close stops every table, aborts in-progress matches, closes sessions and
// waits for the goroutines. Idempotent.
func (r *Registry) Close() error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	for _, t := range r.tables {
		close(t.stop)
	}
	close(r.done)
	r.mu.Unlock()
	r.wg.Wait()
	r.closeSessions() // Task 10
	return nil
}

// Done is closed once Close has signalled every table to stop — before Close
// waits for them — so a Sleep hook that triggers Close can wait for the
// signal without deadlocking on its own goroutine.
func (r *Registry) Done() <-chan struct{} { return r.done }

// ids is the sorted table list every enumeration walks, so no map order
// ever reaches a frame or a file.
func (r *Registry) ids() []TableID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]TableID, 0, len(r.tables))
	for id := range r.tables {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// Stubs the later tasks replace.
func (r *Registry) load() error { return nil }
func (r *Registry) save() error { return nil }
