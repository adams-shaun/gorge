package event_test

import (
	"bytes"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// newSealedEvent creates a sealed event and opens every seat's packs, so
// the phase lands in BUILD. packsPer is the supply size.
func newSealedEvent(t *testing.T, packsPer int) *event.Event {
	t.Helper()
	e := event.New()
	en := event.Entry{Kind: event.KindEventCreated, Seed: 42, Set: "TST", Mode: "sealed", Seats: testSeats()}
	if packsPer != 0 {
		en.Packs = packsPer
	}
	mustApply(t, e, en)
	want := packsPer
	if want == 0 {
		want = event.DefaultSealedPacks
	}
	if e.Mode() != event.ModeSealed || e.PacksPerSeat() != want {
		t.Fatalf("mode %q packs %d, want sealed/%d", e.Mode(), e.PacksPerSeat(), want)
	}
	if packsPer == 0 {
		packsPer = event.DefaultSealedPacks
	}
	sawSupply := false
	for p := 1; p <= packsPer; p++ {
		for s := 0; s < 4; s++ {
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: p, Seat: s, Cards: packCards(p, s)})
			if p == packsPer && s == 3 {
				sawSupply = true
			} else if e.Phase() != event.PhaseDraft {
				t.Fatalf("phase left DRAFT after pack %d seat %d: %s", p, s, e.Phase())
			}
		}
	}
	if !sawSupply {
		t.Fatal("the supply loop opened no packs")
	}
	if e.Phase() != event.PhaseBuild {
		t.Fatalf("phase after the %dth PackOpened is %s, want BUILD", packsPer*4, e.Phase())
	}
	return e
}

// TestSealedEventPoolsAndBuild runs a sealed event end to end: 6 packs per
// seat into private pools, then BUILD, then a round robin to COMPLETE.
func TestSealedEventPoolsAndBuild(t *testing.T) {
	e := newSealedEvent(t, 0) // Packs 0 → the 6-pack default
	for s := 0; s < 4; s++ {
		pool := e.Pool(s)
		if len(pool) != 6*packSize {
			t.Fatalf("seat %d pool has %d cards, want %d", s, len(pool), 6*packSize)
		}
		for p := 1; p <= 6; p++ {
			for i, c := range packCards(p, s) {
				if pool[(p-1)*packSize+i] != c {
					t.Fatalf("seat %d pool position %d is %+v, want pack %d card %d", s, (p-1)*packSize+i, pool[(p-1)*packSize+i], p, i)
				}
			}
		}
		if _, ok := e.Next(s); ok {
			t.Errorf("Next(%d) offered a draft pack in a sealed event", s)
		}
		if q := e.Queued(s); q != 0 {
			t.Errorf("Queued(%d) is %d, want 0 in a sealed event", s, q)
		}
	}
	err := e.Apply(event.Entry{Kind: event.KindPicked, Round: 1, PickNo: 1, Seat: 0, ID: "r1s0c00"})
	if err == nil {
		t.Fatalf("Picked in a sealed event: want error")
	}
	if got := err.Error(); got != "event: Picked: a sealed event has no draft" {
		t.Errorf("sealed Picked error is %q", got)
	}
	saveAndReadyAll(t, e)
	if e.Phase() != event.RoundPhase(1) {
		t.Fatalf("phase after readying is %s, want ROUND1", e.Phase())
	}
	playAllRoundRobin(t, e, scriptedWinner)
	mustApply(t, e, event.Entry{Kind: event.KindEventEnded})
	if !e.Done() {
		t.Fatalf("event ended in phase %s, want COMPLETE", e.Phase())
	}
	st := e.Standings()
	if len(st) != 4 || st[0].Seat != 0 || st[0].Points != 7 {
		t.Fatalf("standings head is %+v, want seat 0 with 7 points", st)
	}
	replayMatches(t, e)

	// The draft-data export has no picks to export: only the header.
	var csv bytes.Buffer
	if err := event.DraftDataCSV(e, &csv, nil); err != nil {
		t.Fatalf("DraftDataCSV: %v", err)
	}
	if got, want := csv.String(), "expansion,event_type,draft_id,pack_number,pick_number,pick\n"; got != want {
		t.Errorf("DraftDataCSV wrote %q, want only the header %q", got, want)
	}
}
