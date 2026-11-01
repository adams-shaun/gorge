package eventhost

import (
	"fmt"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// pumpSweep runs the abandonment sweep by hand until the event reaches phase,
// so a test that relies on several passes does not wait out the real
// sweepEvery ticker between them. now is read fresh on every pass, as the
// ticker's sweep reads Options.Now.
func pumpSweep(t *testing.T, h *Host, id string, now func() time.Time, phase string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		h.checkTimeouts(now())
		st, err := h.EventState(id, "")
		if err != nil {
			t.Fatalf("EventState while waiting for %s: %v", phase, err)
		}
		if st.Phase == phase {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("phase %s not reached in %s (still %s)", phase, within, st.Phase)
		}
		time.Sleep(pollEvery)
	}
}

// TestIdleSavedButUnreadiedDeckAutoCompletes covers the saved-but-unreadied
// half of the idle build rule: a human who saves a deck during BUILD
// and then vanishes without readying must not hold the event in BUILD —
// the sweep readies the saved revision on its own and drives the rest of
// the event to COMPLETE, freeing the -max-events slot.
func TestIdleSavedButUnreadiedDeckAutoCompletes(t *testing.T) {
	clock := newTestClock()
	p := startGamePair(t, "", func(eo *Options, _ *host.Options) {
		eo.MaxHumans = 1
		eo.Now = clock.Now
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	})
	resp := createOK(t, p.h, oneHumanReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")

	// The human plays its whole draft by hand, saves the builder's
	// suggestion, and never readies it.
	pickTop(t, p.h, id, tok, 1<<20)
	pool, err := p.h.EventPool(id, tok)
	if err != nil {
		t.Fatalf("EventPool after draft: %v", err)
	}
	if pool.Ready || pool.Rev != 0 {
		t.Fatalf("precondition: pool Rev %d Ready %v, want 0 false", pool.Rev, pool.Ready)
	}
	if len(pool.Suggestion.Main) == 0 {
		t.Fatal("precondition: builder suggestion is empty")
	}
	if _, err := p.h.SaveDeck(id, tok, protocol.DeckRequest{Main: pool.Suggestion.Main, Basics: pool.Suggestion.Basics}); err != nil {
		t.Fatalf("SaveDeck: %v", err)
	}
	r := runOf(t, p.h, id)
	r.mu.Lock()
	saved, ready := r.ev.SavedRev(0), r.ev.ReadyRev(0)
	r.mu.Unlock()
	if saved != 1 || ready != 0 {
		t.Fatalf("precondition: after save SavedRev %d ReadyRev %d, want 1 and 0", saved, ready)
	}

	// The seat never acts again: the sweep alone must finish BUILD.
	clock.Add(2*time.Hour + time.Minute)
	pumpSweep(t, p.h, id, clock.Now, "COMPLETE", 50*time.Second)

	r.mu.Lock()
	defer r.mu.Unlock()
	readies := 0
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindReady && en.Seat == 0 && en.Rev == 1 {
			readies++
		}
	}
	if readies == 0 {
		t.Error("no sweep ready of seat 0's saved revision 1")
	}
	if st := r.ev.Phase(); st != event.PhaseComplete {
		t.Errorf("phase %s, want COMPLETE", st)
	}
	if n := p.h.liveCount(); n != 0 {
		t.Errorf("live events after COMPLETE: %d, want 0 (slot freed)", n)
	}
}

// TestCheckDeckDoesNotRefreshIdleWindow pins the read-only side of the
// idle rule: polling CheckDeck is not a human action, so a looping client
// cannot keep an otherwise-idle event live past its idle window. Only
// state-changing actions (join, pick, save, ready) refresh the window.
func TestCheckDeckDoesNotRefreshIdleWindow(t *testing.T) {
	clock := newTestClock()
	h, _ := newHost(t, func(o *Options) {
		o.LobbyTimeout = -1
		o.IdleTimeout = 30 * time.Minute
		o.Now = clock.Now
	})
	resp := createOK(t, h, oneHumanReq())
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, h, id, tok, "Ann")
	pickTop(t, h, id, tok, 1)

	// The window closes, then the client polls CheckDeck inside it, then
	// more time passes. The poll must not have reset the window.
	clock.Add(30 * time.Minute)
	for i := 0; i < 3; i++ {
		if _, err := h.CheckDeck(id, tok, protocol.DeckRequest{}); err != nil {
			t.Fatalf("CheckDeck %d: %v", i+1, err)
		}
	}
	clock.Add(5 * time.Minute)
	h.checkTimeouts(clock.Now())

	r := runOf(t, h, id)
	r.mu.Lock()
	defer r.mu.Unlock()
	autos := 0
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPicked && en.Seat == 0 && en.Auto {
			autos++
		}
	}
	if autos == 0 {
		t.Error("seat 0 not auto-picked past its idle window: CheckDeck polls reset lastAct")
	}
}
