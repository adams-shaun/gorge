package host

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
)

// TestOnDemandCapReapsOldestFinished: at the on-demand cap the oldest
// finished private table gives way to the new one (and the hook hears of it);
// a table that has not finished is never removed, so with no finished table
// the add is refused as before.
func TestOnDemandCapReapsOldestFinished(t *testing.T) {
	opts := testOptions(t)
	opts.MaxOnDemandTables = 2
	var mu sync.Mutex
	var removed []TableID
	opts.OnTableRemoved = func(id TableID) {
		mu.Lock()
		removed = append(removed, id)
		mu.Unlock()
	}
	r, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	cfg := func(id TableID) TableConfig {
		return TableConfig{ID: id, Seats: 2, Decks: []string{"a", "b"}, Seed: 7, Spectator: view.Public, OnDemand: true}
	}
	ids := func() []string {
		var out []string
		for _, ti := range r.Tables() {
			out = append(out, ti.ID)
		}
		return out
	}
	seen := func() []TableID {
		mu.Lock()
		defer mu.Unlock()
		return append([]TableID(nil), removed...)
	}
	for _, id := range []TableID{"g1", "g2"} {
		if err := r.AddTable(cfg(id)); err != nil {
			t.Fatal(err)
		}
		if err := r.Start(id); err != nil {
			t.Fatal(err)
		}
		r.Wait(id)
	}
	if got := ids(); !slices.Equal(got, []string{"g1", "g2"}) {
		t.Fatalf("precondition: tables %v, want g1 g2 (both finished, cap reached)", got)
	}
	if err := r.AddTable(cfg("g3")); err != nil {
		t.Fatalf("g3 at the cap with finished tables: %v", err)
	}
	if got := ids(); !slices.Equal(got, []string{"g2", "g3"}) {
		t.Fatalf("after g3: tables %v, want g2 g3 (oldest finished g1 reaped)", got)
	}
	if got := seen(); !slices.Equal(got, []TableID{"g1"}) {
		t.Fatalf("hook saw %v, want [g1]", got)
	}
	if _, err := r.Matches("g1"); err != ErrNotFound {
		t.Fatalf("Matches(g1) after reap = %v, want ErrNotFound", err)
	}
	// g3 is registered but never finished: only g2 can give way.
	if err := r.AddTable(cfg("g4")); err != nil {
		t.Fatalf("g4: %v", err)
	}
	if got := ids(); !slices.Equal(got, []string{"g3", "g4"}) {
		t.Fatalf("after g4: tables %v, want g3 g4", got)
	}
	// g3 and g4 are both still live: nothing gives way.
	if err := r.AddTable(cfg("g5")); err == nil || !strings.Contains(err.Error(), "on-demand table limit 2") {
		t.Fatalf("g5 with no finished table: err %v, want the limit refusal", err)
	}
	if got := ids(); !slices.Equal(got, []string{"g3", "g4"}) {
		t.Fatalf("refusal removed a live table: %v", got)
	}
	if got := seen(); !slices.Equal(got, []TableID{"g1", "g2"}) {
		t.Fatalf("hook saw %v, want [g1 g2]", got)
	}
}
