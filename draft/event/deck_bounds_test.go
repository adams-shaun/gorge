package event_test

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// eventAtBuild drives a fresh scripted event through the draft to the BUILD
// phase, where DeckSaved is legal, and returns it.
func eventAtBuild(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 7, Set: "TST", Seats: testSeats()})
	for r := 1; r <= draft.Rounds; r++ {
		for s := 0; s < draft.Seats; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: packCards(r, s)})
		}
	}
	for e.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < draft.Seats; s++ {
			v, ok := e.Next(s)
			if !ok {
				continue
			}
			progressed = true
			mustApply(t, e, event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: v.Pack[0].ID, Auto: true})
		}
		if !progressed {
			t.Fatalf("draft stalled in phase %s", e.Phase())
		}
	}
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("event reached phase %s, want BUILD", e.Phase())
	}
	return e
}

// TestDeckSavedRejectsOutOfBoundsDecks proves the event path inherits the
// hosting bounds of build.Validate: a negative basic, 41 of one
// basic, a 101-card deck and an overflowing count are all rejected, while a
// legal deck is still accepted.
func TestDeckSavedRejectsOutOfBoundsDecks(t *testing.T) {
	e := eventAtBuild(t)
	pool := e.Pool(0)
	if len(pool) < 42 {
		t.Fatalf("seat 0 pool has %d cards, want at least 42", len(pool))
	}
	mainOf := func(n int) []string {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = pool[i].ID
		}
		return ids
	}

	cases := []struct {
		name string
		deck build.Deck
	}{
		// 41 main + a negative basic: total 40, so only the negative count
		// is wrong.
		{"negative basic", build.Deck{Main: mainOf(41), Basics: [5]int{-1, 0, 0, 0, 0}}},
		{"41 of one basic", build.Deck{Basics: [5]int{41, 0, 0, 0, 0}}},
		// 42 main + 30+29 basics = 101, each type within the per-type cap.
		{"101-card deck", build.Deck{Main: mainOf(42), Basics: [5]int{30, 29, 0, 0, 0}}},
		{"overflowing count", build.Deck{Basics: [5]int{1 << 62, 0, 0, 0, 0}}},
	}
	for _, tc := range cases {
		if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1, Deck: &tc.deck}); err == nil {
			t.Errorf("DeckSaved with %s: want error, got nil", tc.name)
		}
	}

	// The handler still accepts a legal deck, so the rejections above are
	// the bounds and not a broken apply path.
	legal := build.Deck{Main: mainOf(35), Basics: [5]int{5, 0, 0, 0, 0}}
	if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: 0, Rev: 1, Deck: &legal}); err != nil {
		t.Fatalf("DeckSaved with a legal 40-card deck: %v", err)
	}
	if _, ok := e.SavedDeck(0); !ok {
		t.Fatal("a legal DeckSaved did not record a revision")
	}
}
