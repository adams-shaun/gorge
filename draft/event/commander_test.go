package event_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
)

// commanderPacks builds 4 seats × 3 rounds of synthetic 20-card packs with
// event-unique IDs.
func commanderPacks() [4][3][]limited.CardRef {
	var out [4][3][]limited.CardRef
	for s := 0; s < 4; s++ {
		for r := 0; r < 3; r++ {
			for i := 0; i < 20; i++ {
				id := fmt.Sprintf("s%dr%dc%02d", s, r, i)
				out[s][r] = append(out[s][r], limited.CardRef{ID: id, Name: "Card " + id, Set: "TST", Number: "1", Rarity: limited.Common})
			}
		}
	}
	return out
}

// applyAll applies entries, failing the test on the first refusal.
func applyAll(t *testing.T, e *event.Event, ens ...event.Entry) {
	t.Helper()
	for _, en := range ens {
		if err := e.Apply(en); err != nil {
			t.Fatalf("Apply %s: %v", en.Kind, err)
		}
	}
}

// TestCommanderPodEvent runs a Commander-draft event end to end on synthetic
// 20-card packs: pick two per pass, 60-card pools, 60-card decks naming a
// commander, one free-for-all game, the standings, and a byte-identical
// replay. It also pins the shape refusals.
func TestCommanderPodEvent(t *testing.T) {
	seats := make([]event.SeatSpec, 4)
	for i := range seats {
		seats[i] = event.SeatSpec{Kind: "bot", Name: fmt.Sprintf("bot%d", i)}
	}
	created := event.Entry{Kind: event.KindEventCreated, Seed: 9, Set: "TST", Seats: seats,
		Format: event.FormatCommander, Structure: event.StructurePod, PicksPerPass: 2, Rounds: 2}

	for _, bad := range []event.Entry{
		{Kind: event.KindEventCreated, Seed: 9, Set: "TST", Seats: seats, Structure: event.StructurePod},
		{Kind: event.KindEventCreated, Seed: 9, Set: "TST", Seats: seats, Format: event.FormatCommander},
		{Kind: event.KindEventCreated, Seed: 9, Set: "TST", Seats: seats, Format: event.FormatCommander, Structure: event.StructurePod, PicksPerPass: 9},
	} {
		if err := event.New().Apply(bad); err == nil {
			t.Errorf("EventCreated %+v accepted, want a refusal", bad)
		}
	}

	e := event.New()
	applyAll(t, e, created)
	packs := commanderPacks()
	for s := 0; s < 4; s++ {
		for r := 0; r < 3; r++ {
			applyAll(t, e, event.Entry{Kind: event.KindPackOpened, Round: r + 1, Seat: s, Cards: packs[s][r]})
		}
	}
	for e.Phase() == event.PhaseDraft {
		for s := 0; s < 4; s++ {
			if v, ok := e.Next(s); ok {
				applyAll(t, e, event.Entry{Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: v.Pack[0].ID})
			}
		}
	}
	for s := 0; s < 4; s++ {
		pool := e.Pool(s)
		if len(pool) != 60 {
			t.Fatalf("seat %d pool = %d cards, want 60", s, len(pool))
		}
		var main []string
		for _, c := range pool[:40] {
			main = append(main, c.ID)
		}
		limitedDeck := build.Deck{Main: main[:23], Basics: [5]int{17}}
		if err := e.Apply(event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &limitedDeck}); err == nil {
			t.Fatalf("seat %d: a 40-card deck with no commander was saved in a Commander event", s)
		}
		d := build.Deck{Main: main, Basics: [5]int{0, 0, 0, 20, 0}, Commanders: []string{main[0], main[1]}}
		applyAll(t, e,
			event.Entry{Kind: event.KindDeckSaved, Seat: s, Rev: 1, Deck: &d},
			event.Entry{Kind: event.KindReady, Seat: s, Rev: 1})
	}
	if len(e.PendingSeries()) != 0 {
		t.Fatalf("a pod event has series pending: %v", e.PendingSeries())
	}
	for game, winner := range []*int{intp(2), nil} {
		round, ok := e.PendingPodGame()
		if !ok || round != game+1 {
			t.Fatalf("PendingPodGame = %d, %v; want %d, true", round, ok, game+1)
		}
		applyAll(t, e, event.Entry{Kind: event.KindPodGameEnded, Round: round, Winner: winner, Head: fmt.Sprintf("head%d", round)})
	}
	applyAll(t, e, event.Entry{Kind: event.KindEventEnded})
	st := e.Standings()
	if st[0].Seat != 2 || st[0].Points != 4 || st[1].Points != 1 {
		t.Fatalf("standings %+v, want seat 2 first on 4 points (a win and a draw), the rest on 1", st)
	}

	wire, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatal(err)
	}
	again, err := event.ReplayLog(wire)
	if err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	a, _ := e.StateJSON()
	b, _ := again.StateJSON()
	if !bytes.Equal(a, b) {
		t.Fatalf("replayed state differs:\n%s\n%s", a, b)
	}
}

func intp(v int) *int { return &v }
