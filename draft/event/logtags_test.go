package event_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestOldLogReplaysAfterTagChange proves logs written before E1-8's
// snake_case tags (capitalised Main/Basics, a seats block on every line)
// still parse, replay and rebuild the same event: JSON field decoding is
// case-insensitive, so a stored log from an older binary survives the tag
// change.
func TestOldLogReplaysAfterTagChange(t *testing.T) {
	e := newScriptedEvent(t)
	entries := e.Log()
	wire, err := event.MarshalLog(entries)
	if err != nil {
		t.Fatal(err)
	}

	// Rewrite the wire into the pre-E1-8 spelling: every entry carries
	// "seats":[{},{},{},{}] (the old field had no omitempty) and the deck
	// object's keys are capitalised.
	lines := strings.Split(strings.TrimRight(string(wire), "\n"), "\n")
	oldLines := make([]string, 0, len(lines))
	for i, line := range lines {
		if i > 0 {
			line = strings.Replace(line, "{", `{"seats":[{},{},{},{}],`, 1)
		}
		line = strings.ReplaceAll(line, `"main":`, `"Main":`)
		line = strings.ReplaceAll(line, `"basics":`, `"Basics":`)
		oldLines = append(oldLines, line)
	}
	legacy := []byte(strings.Join(oldLines, "\n") + "\n")

	parsed, err := event.ParseLog(legacy)
	if err != nil {
		t.Fatalf("event.ParseLog(legacy): %v", err)
	}
	sawDeck := false
	for _, en := range parsed {
		if en.Kind == event.KindDeckSaved {
			if en.Deck == nil || len(en.Deck.Main) == 0 {
				t.Fatalf("legacy deck for seat %d did not decode", en.Seat)
			}
			sawDeck = true
		}
	}
	if !sawDeck {
		t.Fatal("no DeckSaved entry in the fixture")
	}

	e2, err := event.Replay(parsed)
	if err != nil {
		t.Fatalf("event.Replay(legacy): %v", err)
	}
	if e2.Phase() != event.PhaseComplete {
		t.Fatalf("replayed phase %s, want COMPLETE", e2.Phase())
	}
	d0, ok := e2.SavedDeck(0)
	if !ok {
		t.Fatal("replayed event lost seat 0's saved deck")
	}
	want, ok := e.SavedDeck(0)
	if !ok {
		t.Fatal("source event lost seat 0's saved deck")
	}
	if !reflect.DeepEqual(d0, want) {
		t.Errorf("seat 0 deck %+v, want %+v", d0, want)
	}
	if got, want := e2.Seats(), e.Seats(); !reflect.DeepEqual(got, want) {
		t.Errorf("seats %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(e2.Standings(), e.Standings()) {
		t.Errorf("standings diverged")
	}

	// The new spelling round-trips entry for entry.
	again, err := event.ParseLog(wire)
	if err != nil {
		t.Fatalf("event.ParseLog(wire): %v", err)
	}
	if !reflect.DeepEqual(again, entries) {
		t.Errorf("new spelling does not round-trip")
	}
}
