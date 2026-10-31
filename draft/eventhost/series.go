package eventhost

import (
	"context"
	"sort"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/host"
)

// waiter is one series table's hook call parked on its run: the match
// number the table asked for and the buffered channel its answer arrives
// on. One table goroutine asks at a time, so the map holds at most one
// waiter per live series table.
type waiter struct {
	k  int
	ch chan seriesAnswer
}

// seriesAnswer is one parked hook call's answer: the plan for match k (ok)
// or the instruction to end the table (!ok). Both are (zero, false) when
// only the channel matters (a Host.Close release).
type seriesAnswer struct {
	g  host.SeriesGame
	ok bool
}

// Series is the Host's host.SeriesFunc: it plans match k of a series table
// the run started (E3-4). It reads the event's state and answers ready
// with the in-flight game's decks and play/draw chooser, stop when the
// series has ended or moved to another table, or — between games, before
// advanceRounds has logged the next GameStarted — parks on the run until
// wakeWaiters answers it. It never applies a log entry and never calls the
// Registry: the log is a function of the event's own state, never of when
// the table goroutine reached the hook. Lock order: h.mu alone to find the
// ref, then r.mu alone; it waits holding nothing.
func (h *Host) Series(ctx context.Context, t host.TableID, k int) (host.SeriesGame, bool, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return host.SeriesGame{}, false, nil
	}
	ref, ok := h.tables[t]
	h.mu.Unlock()
	if !ok || !ref.series {
		return host.SeriesGame{}, false, nil
	}
	r := ref.run
	r.mu.Lock()
	g, ok, wait := r.seriesPlan(ref, t, k)
	if !wait {
		r.mu.Unlock()
		if !ok {
			// The series is over: drop the table's ref after answering
			// stop, so a late OnMatchEnd is dropped by the lookup.
			h.mu.Lock()
			delete(h.tables, t)
			h.mu.Unlock()
		}
		return g, ok, nil
	}
	ch := make(chan seriesAnswer, 1)
	r.waiters[t] = waiter{k: k, ch: ch}
	// A Close between the entry check and this registration ran its
	// releaseWaiters loop without a waiter of ours to release: re-check
	// closed under h.mu and answer stop, so the call never parks under a
	// closed host.
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		delete(r.waiters, t)
		r.mu.Unlock()
		h.mu.Lock()
		delete(h.tables, t)
		h.mu.Unlock()
		return host.SeriesGame{}, false, nil
	}
	r.mu.Unlock()
	select {
	case a := <-ch:
		return a.g, a.ok, nil
	case <-ctx.Done():
		r.mu.Lock()
		if cur, ok := r.waiters[t]; ok && cur.ch == ch {
			delete(r.waiters, t)
		}
		r.mu.Unlock()
		return host.SeriesGame{}, false, nil
	}
}

// seriesPlan computes the hook's answer for match k of the series table t.
// The caller holds r.mu. ok=true is a ready plan, ok=false a stop, wait
// "park on the run until the log moves".
func (r *run) seriesPlan(ref gameRef, t host.TableID, k int) (host.SeriesGame, bool, bool) {
	game := ref.firstGame + k - 1
	v, ok := r.ev.SeriesState(ref.a, ref.b)
	if !ok || v.Round != ref.round {
		return host.SeriesGame{}, false, false
	}
	if v.InFlight != nil && v.InFlight.Table == string(t) && v.InFlight.Game == game {
		chooser := -1
		if game > 1 {
			// The plan's Chooser is a table slot: slot 0 plays event seat
			// a, slot 1 event seat b (series plans are never rotated).
			chooser = 0
			if v.InFlight.Chooser == ref.b {
				chooser = 1
			}
		}
		return host.SeriesGame{
			Decks: []string{
				DeckID(r.meta.ID, ref.a, v.PlayRevs[0]),
				DeckID(r.meta.ID, ref.b, v.PlayRevs[1]),
			},
			Chooser: chooser,
		}, true, false
	}
	if v.Ended || r.seriesTable[[2]int{ref.a, ref.b}] != t {
		return host.SeriesGame{}, false, false
	}
	return host.SeriesGame{}, false, true
}

// wakeWaiters answers every parked hook call the log has unblocked. It is
// called at the end of every advance under r.mu. Waiters are answered in
// sorted table id order, so map iteration order never reaches an answer.
// A waiter the log has still not unblocked is put back; the rest are sent
// their ready or stop answer and dropped. A stop answer also drops the
// table's ref, so a late match result is dropped by OnMatchEnd's lookup.
func (r *run) wakeWaiters() {
	if len(r.waiters) == 0 {
		return
	}
	ids := make([]host.TableID, 0, len(r.waiters))
	for id := range r.waiters {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		w := r.waiters[id]
		delete(r.waiters, id)
		r.h.mu.Lock()
		ref, ok := r.h.tables[id]
		r.h.mu.Unlock()
		if !ok {
			w.ch <- seriesAnswer{}
			continue
		}
		g, ready, wait := r.seriesPlan(ref, id, w.k)
		if wait {
			r.waiters[id] = w
			continue
		}
		if !ready {
			r.h.mu.Lock()
			delete(r.h.tables, id)
			r.h.mu.Unlock()
		}
		w.ch <- seriesAnswer{g: g, ok: ready}
	}
}

// tableSeed is one live series table's game seed, read back from the
// log's GameStarted entries naming it — every entry that names it carries
// the table's seed, since a game continuing on it logs GameStarted again
// with the same seed. ok=false means the table has none — a game 1/2
// whose GameStarted logged but whose table row the caller lost track of.
func (r *run) tableSeed(id host.TableID) (uint64, bool) {
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindGameStarted && en.Table == string(id) {
			return en.GameSeed, true
		}
	}
	return 0, false
}

// releaseWaiters answers every parked hook call stop, unblocking the
// series tables so Host.Close's wg.Wait cannot wait on them. The caller
// holds r.mu.
func (r *run) releaseWaiters() {
	ids := make([]host.TableID, 0, len(r.waiters))
	for id := range r.waiters {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		w := r.waiters[id]
		delete(r.waiters, id)
		w.ch <- seriesAnswer{}
	}
}
