package host

// The sideboard-less compatibility half of the sideboard feature (task
// sideboards, fix round 3): rules.Config.Sideboards must be nil — not a
// non-nil slice of empty seats — when no deck at the table carries a
// sideboard, or FeedbackMatch's `sideboards,omitempty` cannot omit it and
// every ordinary match.json grows a `[null,...]` field it never had.
// newMatch used to allocate the slice unconditionally, so this pins the
// shape at the SITE that builds the Config from loaded decks, and at the
// capture that serialises it.

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// sideboardTestLoader serves SampleDecks' four sideboard-less decks under
// "a".."d", with deck "a" additionally carrying one sideboard card when
// withSideboard is true. The card is a synthetic land, so the test is
// corpus-free and can run beside every other host test.
func sideboardTestLoader(t *testing.T, withSideboard bool) func(string) (Deck, error) {
	t.Helper()
	names, decks := testutil.SampleDecks(t, 4)
	byName := map[string][]*cards.Card{}
	for i, n := range names {
		byName[n] = decks[i]
	}
	return func(name string) (Deck, error) {
		cs, ok := byName[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		d := Deck{Name: name, Cards: cs}
		if withSideboard && name == "a" {
			d.Sideboard = []*cards.Card{cs[0]}
		}
		return d, nil
	}
}

// TestNewMatchDropsAnAllEmptySideboard proves the class fix: a table whose
// decks carry no sideboard gets a nil Config.Sideboards (so capture omits
// the key), while a table whose deck DOES carry one keeps the per-seat
// list and captures it. Both branches run, so an implementation that
// simply always nils the field fails the second assertion.
func TestNewMatchDropsAnAllEmptySideboard(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		withSideboard bool
		wantNil       bool
		wantKey       bool
	}{
		{name: "no-deck-has-one", withSideboard: false, wantNil: true, wantKey: false},
		{name: "one-deck-has-one", withSideboard: true, wantNil: false, wantKey: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := testOptions(t)
			opts.LoadDeck = sideboardTestLoader(t, tc.withSideboard)
			r, err := New(opts)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			if err := r.AddTable(TableConfig{ID: "t1", Name: "sideboard", Seats: 1,
				Decks: []string{"a"}, Seed: 7, Spectator: view.Omniscient}); err != nil {
				t.Fatal(err)
			}
			if err := r.Start("t1"); err != nil {
				t.Fatal(err)
			}
			r.Wait("t1")

			r.mu.RLock()
			tb := r.tables["t1"]
			r.mu.RUnlock()
			if tb == nil || len(tb.history) == 0 {
				t.Fatal("no finished match after Start")
			}
			m := tb.history[0]
			m.mu.RLock()
			gotNil := m.cfg.Sideboards == nil
			// The live match's own sideboard zone must agree with the
			// Config: no cards means an empty zone, one card means the
			// card is really seated there.
			gotZone := len(m.e.G.Zone(state.ZSideboard, 0))
			m.mu.RUnlock()
			if gotNil != tc.wantNil {
				t.Errorf("Config.Sideboards nil = %v, want %v (got %#v)",
					gotNil, tc.wantNil, m.cfg.Sideboards)
			}
			wantZone := 0
			if tc.withSideboard {
				wantZone = 1
			}
			if gotZone != wantZone {
				t.Errorf("sideboard zone holds %d cards, want %d", gotZone, wantZone)
			}

			snap, err := r.SnapshotForFeedback("t1", nil)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(snap.Match)
			if err != nil {
				t.Fatal(err)
			}
			var keyed map[string]any
			if err := json.Unmarshal(raw, &keyed); err != nil {
				t.Fatal(err)
			}
			_, hasKey := keyed["sideboards"]
			if hasKey != tc.wantKey {
				t.Errorf("match.json has sideboards key = %v, want %v\n%s",
					hasKey, tc.wantKey, raw)
			}
			if tc.wantKey {
				sb, ok := keyed["sideboards"].([]any)
				if !ok || len(sb) != 1 {
					t.Fatalf("sideboards = %#v, want one seat list", keyed["sideboards"])
				}
				seat, ok := sb[0].([]any)
				if !ok || len(seat) != 1 {
					t.Fatalf("seat 0 sideboard = %#v, want one card name", sb[0])
				}
			}
		})
	}
}
