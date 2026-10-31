package event_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestDraftDataCSVWithOptionsPerSeat exercises the option form: a custom
// event_type and one draft_id per drafter ("<seed>-s<seat>") reach every
// row, the header shape is unchanged, and the plain entry points stay
// byte-identical to the option form's defaults.
func TestDraftDataCSVWithOptionsPerSeat(t *testing.T) {
	e := newCSVEvent(t)
	names := []string{"Gamma", "Alpha", "Beta"}
	var buf bytes.Buffer
	if err := event.DraftDataCSVWith(e, &buf, names, event.DraftDataOptions{
		EventType:      "test_type",
		PerSeatDraftID: true,
		Header:         true,
	}); err != nil {
		t.Fatalf("DraftDataCSVWith: %v", err)
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
	// The pick and count columns are untouched by the options; only the
	// event_type and draft_id cells move.
	want1 := "TST,test_type,7-s0,1,1,Alpha,1,1,0,0,0,0"
	if lines[1] != want1 {
		t.Errorf("row 1 is %q, want %q", lines[1], want1)
	}
	want2 := "TST,test_type,7-s1,1,1,Gamma,1,0,1,0,0,0"
	if lines[2] != want2 {
		t.Errorf("row 2 is %q, want %q", lines[2], want2)
	}
	// The fixture's picks were applied seat 0,1,2,3 cycling, so row i's
	// drafter is seat i%4 and every drafter picked 6 times.
	counts := map[string]int{}
	for i, line := range lines[1:] {
		f := strings.Split(line, ",")
		if f[1] != "test_type" {
			t.Errorf("row %d event_type %q, want test_type", i, f[1])
		}
		wantID := fmt.Sprintf("7-s%d", i%4)
		if f[2] != wantID {
			t.Errorf("row %d draft_id %q, want %q", i, f[2], wantID)
		}
		counts[f[2]]++
	}
	if len(counts) != 4 {
		t.Fatalf("%d distinct draft ids, want 4", len(counts))
	}
	for id, n := range counts {
		if n != 6 {
			t.Errorf("draft id %s has %d rows, want 6", id, n)
		}
	}
}

// TestDraftDataCSVWithOptionsDefaults pins the two plain entry points
// against the option form: DraftDataCSV is With({Header: true}) and
// DraftDataCSVRows is With({}) byte for byte.
func TestDraftDataCSVWithOptionsDefaults(t *testing.T) {
	e := newCSVEvent(t)
	names := []string{"Gamma", "Alpha", "Beta"}
	var legacy, opt bytes.Buffer
	if err := event.DraftDataCSV(e, &legacy, names); err != nil {
		t.Fatalf("DraftDataCSV: %v", err)
	}
	if err := event.DraftDataCSVWith(e, &opt, names, event.DraftDataOptions{Header: true}); err != nil {
		t.Fatalf("DraftDataCSVWith: %v", err)
	}
	if !bytes.Equal(legacy.Bytes(), opt.Bytes()) {
		t.Errorf("DraftDataCSV differs from DraftDataCSVWith({Header: true})")
	}

	legacy.Reset()
	opt.Reset()
	if err := event.DraftDataCSVRows(e, &legacy, names); err != nil {
		t.Fatalf("DraftDataCSVRows: %v", err)
	}
	if err := event.DraftDataCSVWith(e, &opt, names, event.DraftDataOptions{}); err != nil {
		t.Fatalf("DraftDataCSVWith: %v", err)
	}
	if !bytes.Equal(legacy.Bytes(), opt.Bytes()) {
		t.Errorf("DraftDataCSVRows differs from DraftDataCSVWith({})")
	}
}

// TestDraftDataCSVWithOptionsSealed: a created-but-sealed event carries no
// picks, so the option form writes the header only (or nothing without
// one), like the plain entry points.
func TestDraftDataCSVWithOptionsSealed(t *testing.T) {
	e := event.New()
	mustApply(t, e, event.Entry{Kind: event.KindEventCreated, Seed: 7, Set: "TST", Seats: []event.SeatSpec{
		{Kind: "bot", Name: "a"}, {Kind: "bot", Name: "b"}, {Kind: "bot", Name: "c"}, {Kind: "bot", Name: "d"},
	}})
	var buf bytes.Buffer
	if err := event.DraftDataCSVWith(e, &buf, []string{"Alpha"}, event.DraftDataOptions{Header: true}); err != nil {
		t.Fatalf("DraftDataCSVWith: %v", err)
	}
	if got := strings.Count(buf.String(), "\n"); got != 1 {
		t.Fatalf("sealed CSV has %d lines, want one header", got)
	}
	buf.Reset()
	if err := event.DraftDataCSVWith(e, &buf, []string{"Alpha"}, event.DraftDataOptions{}); err != nil {
		t.Fatalf("DraftDataCSVWith: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("sealed rows-only CSV wrote %q, want empty", buf.String())
	}
}
