package eventhost

import (
	"fmt"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestResumeWhileHumanSideboards stops the host and the registry (a crash)
// while a human seat sits between the games of its Bo3 series, reopens
// everything from the same persistence directory and shows the seat still
// between games with its Sideboard ask outstanding — then that its keep
// starts game 2 on a fresh table (…-g2-a1), not the pre-restart one.
func TestResumeWhileHumanSideboards(t *testing.T) {
	dir := t.TempDir()
	mut := func(eo *Options, _ *host.Options) {
		eo.MaxHumans = 1
		eo.IdleTimeout = -1
		eo.MintSeat = func(table host.TableID, seat int) (string, error) {
			return fmt.Sprintf("tok-%s-%d", table, seat), nil
		}
	}
	p := startGamePair(t, dir, mut)
	req := oneHumanReq()
	req.BestOf = 3
	resp := createOK(t, p.h, req)
	id := resp.Event
	tok, _ := humanToken(t, resp)
	joinOK(t, p.h, id, tok, "Ann")
	pickTop(t, p.h, id, tok, 45)
	finishHumanBuild(t, p.h, id, tok)
	waitPhase(t, p.h, id, "ROUND1", 50*time.Second)

	r := runOf(t, p.h, id)
	humanDriveUntilBetween(t, p.reg, r, 0, 3)
	r.mu.Lock()
	v, ok := r.ev.SeriesState(0, 3)
	oldTable := r.seriesTable[[2]int{0, 3}]
	r.mu.Unlock()
	if !ok || !v.Between || v.NextGame != 2 || v.Locked != [2]int{0, 0} {
		t.Fatalf("series before the stop = %+v, want between game 2, both sides unlocked", v)
	}
	if oldTable == "" {
		t.Fatal("precondition: no series table before the stop")
	}

	// The crash: everything for this event stops dead.
	p.h.Close()
	p.reg.Close()

	// The restart reopens the same directory; the series is still between
	// games with seat 0's sideboard ask outstanding.
	p2 := startGamePair(t, dir, mut)
	r2 := runOf(t, p2.h, id)
	r2.mu.Lock()
	v2, ok2 := r2.ev.SeriesState(0, 3)
	r2.mu.Unlock()
	if !ok2 || !v2.Between || v2.NextGame != 2 {
		t.Fatalf("series after resume = %+v, want still between game 2", v2)
	}
	st := stateOf(t, p2.h, id, tok)
	row := seriesRow(st, 0, 3)
	if row == nil || row.State != "between" || row.Game != 2 {
		t.Fatalf("series row after resume = %+v, want between game 2", row)
	}
	sawSelf := false
	for _, w := range row.Waiting {
		if w == 0 {
			sawSelf = true
		}
	}
	if !sawSelf {
		t.Fatalf("series row after resume = %+v, want seat 0 still in Waiting (ask outstanding)", row)
	}

	// Seat 0 keeps: game 2 starts on a fresh table (…-g2-a1), never the
	// pre-restart one.
	if _, err := p2.h.Sideboard(id, tok, protocol.SideboardRequest{Game: 2, Keep: true}); err != nil {
		t.Fatalf("Sideboard(keep) after resume: %v", err)
	}
	att, table := waitSeriesInFlight(t, r2, 0, 3, 2, 50*time.Second)
	want := gameTableID(id, 1, 0, 3, 2, 1)
	if table != want {
		t.Fatalf("resumed game 2 table %q, want %q", table, want)
	}
	if host.TableID(att.Table) != want {
		t.Fatalf("game 2 attempt %+v names table %q", att, want)
	}
	if oldTable == want {
		t.Fatalf("game 2 reused the pre-restart table %s", oldTable)
	}
}
