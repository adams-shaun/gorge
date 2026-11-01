package event_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// csvCard builds one named fake card for the CSV fixture.
func csvCard(id, name string) limited.CardRef {
	return limited.CardRef{ID: id, Name: name, Set: "TST", Number: id, Rarity: limited.Common}
}

// newCSVEvent drafts a tiny 4-seat event whose packs hold two cards each,
// picking every card, and returns the completed-through-draft event.
func newCSVEvent(t *testing.T) *event.Event {
	t.Helper()
	e := event.New()
	seats := []event.SeatSpec{{Kind: "bot", Name: "a"}, {Kind: "bot", Name: "b"}, {Kind: "bot", Name: "c"}, {Kind: "bot", Name: "d"}}
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 7, Set: "TST", Seats: seats})
	names := [][]string{
		{"Alpha", "Beta"},  // round 1: seat 0's pack
		{"Gamma", "Alpha"}, // round 1: seat 1's pack
		{"Beta", "Gamma"},  // round 1: seat 2's pack
		{"Alpha", "Gamma"}, // round 1: seat 3's pack
		{"Beta", "Alpha"},  // round 2: seat 0's pack
		{"Gamma", "Beta"},  // round 2: seat 1's pack
		{"Alpha", "Gamma"}, // round 2: seat 2's pack
		{"Beta", "Alpha"},  // round 2: seat 3's pack
		{"Gamma", "Beta"},  // round 3: seat 0's pack
		{"Alpha", "Beta"},  // round 3: seat 1's pack
		{"Gamma", "Alpha"}, // round 3: seat 2's pack
		{"Beta", "Gamma"},  // round 3: seat 3's pack
	}
	for r := 1; r <= draft.Rounds; r++ {
		for s := 0; s < draft.Seats; s++ {
			pack := []limited.CardRef{
				csvCard(fmtID(r, s, 0), names[(r-1)*4+s][0]),
				csvCard(fmtID(r, s, 1), names[(r-1)*4+s][1]),
			}
			mustApply(t, e, event.Entry{Kind: event.KindPackOpened, Round: r, Seat: s, Cards: pack})
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
	return e
}

// fmtID is the fixture's card ID scheme.
func fmtID(r, s, i int) string {
	return fmt.Sprintf("r%ds%dc%d", r, s, i)
}

// TestDraftDataCSV checks the 17Lands draft_data header and the first two
// rows of a tiny 4-card fixture.
func TestDraftDataCSV(t *testing.T) {
	e := newCSVEvent(t)
	var buf bytes.Buffer
	if err := event.DraftDataCSV(e, &buf, []string{"Gamma", "Alpha", "Beta"}); err != nil {
		t.Fatalf("DraftDataCSV: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 1+24 {
		t.Fatalf("CSV has %d lines, want header plus 24 picks", len(lines))
	}
	wantHeader := "expansion,event_type,draft_id,pack_number,pick_number,pick," +
		"pack_card_Alpha,pack_card_Beta,pack_card_Gamma," +
		"pool_Alpha,pool_Beta,pool_Gamma"
	if lines[0] != wantHeader {
		t.Errorf("header is %q, want %q", lines[0], wantHeader)
	}

	// Row 1: seat 0's first pick. Its round-1 pack is Alpha,Beta; the pool
	// is empty and the pick is Alpha.
	want1 := "TST,gorge_pod,7,1,1,Alpha,1,1,0,0,0,0"
	if lines[1] != want1 {
		t.Errorf("row 1 is %q, want %q", lines[1], want1)
	}
	// Row 2: seat 1's first pick. Its round-1 pack is Gamma,Alpha; the pool
	// is empty and the pick is Gamma.
	want2 := "TST,gorge_pod,7,1,1,Gamma,1,0,1,0,0,0"
	if lines[2] != want2 {
		t.Errorf("row 2 is %q, want %q", lines[2], want2)
	}
}

// TestDraftDataCSVUncreated refuses an event without its EventCreated.
func TestDraftDataCSVUncreated(t *testing.T) {
	var buf bytes.Buffer
	if err := event.DraftDataCSV(event.New(), &buf, nil); err == nil {
		t.Errorf("DraftDataCSV on an uncreated event: want error")
	}
}
