package view

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestOmniscientRecordNotForSeats pins the E8 producer: the record holds every
// seat's library in true order, a playing seat (the zero RecordCaller) is
// refused, and neither producing a record nor the record's own View changes
// what a Seat or Omniscient projection shows.
func TestOmniscientRecordNotForSeats(t *testing.T) {
	g := visibilityBoard(t)
	ch := flatChars{g}
	const seat = state.PlayerID(0)

	// Make each library's order distinguishable from its sorted (name, id)
	// order: reverse seat 1's ids, so a producer that sorted would fail.
	lib := g.Zone(state.ZLibrary, 1)
	rev := make([]state.ObjID, len(lib))
	for i := range lib {
		rev[len(lib)-1-i] = lib[i]
	}
	g.SetZone(state.ZLibrary, 1, rev)
	if len(rev) < 2 || rev[0] < rev[1] {
		t.Fatalf("fixture precondition: seat 1's library must be in descending id order, got %v", rev)
	}

	seatBefore := ProjectFor(g, ch, seat, Seat, nil)
	omniBefore := ProjectFor(g, ch, seat, Omniscient, nil)

	// A playing seat is refused, with no record.
	rec, err := ProduceOmniscientRecord(g, ch, seat, RecordForSeat, nil)
	if !errors.Is(err, ErrRecordForSeat) {
		t.Fatalf("seat caller: err = %v, want ErrRecordForSeat", err)
	}
	if rec.Libraries != nil || rec.View.Players != nil {
		t.Fatalf("a refused request still returned data: %+v", rec)
	}
	if _, err := ProduceOmniscientRecord(g, ch, seat, RecordCaller(200), nil); !errors.Is(err, ErrRecordForSeat) {
		t.Fatalf("unknown caller: err = %v, want ErrRecordForSeat", err)
	}

	for _, caller := range []RecordCaller{RecordForTraining, RecordForDiagnostics} {
		rec, err := ProduceOmniscientRecord(g, ch, seat, caller, nil)
		if err != nil {
			t.Fatalf("caller %d: %v", caller, err)
		}
		if rec.View.Visibility != "omniscient" || !reflect.DeepEqual(rec.View, omniBefore) {
			t.Fatalf("caller %d: record's View is not the Omniscient projection", caller)
		}
		if len(rec.Libraries) != len(g.Players) {
			t.Fatalf("caller %d: %d libraries, want %d", caller, len(rec.Libraries), len(g.Players))
		}
		for p := range g.Players {
			assertZoneViewsMatch(t, "record library", state.PlayerID(p), g.Zone(state.ZLibrary, state.PlayerID(p)), rec.Libraries[p])
		}
		// The record's order differs from the unordered own-library list the
		// Seat view carries, so it holds information the seat view lacks.
		if rec.Libraries[1][0].ID == sortedFirst(rec.Libraries[1]) {
			t.Fatalf("precondition: seat 1's top card is also its lowest id; order proves nothing")
		}
	}

	// The record is the only place the order lives: no projection mentions it.
	seatAfter := ProjectFor(g, ch, seat, Seat, nil)
	omniAfter := ProjectFor(g, ch, seat, Omniscient, nil)
	if !reflect.DeepEqual(seatBefore, seatAfter) {
		t.Fatal("producing a record changed the Seat projection")
	}
	if !reflect.DeepEqual(omniBefore, omniAfter) {
		t.Fatal("producing a record changed the Omniscient projection")
	}
	for p := range omniAfter.Players {
		pv := omniAfter.Players[p]
		if pv.Library != nil || pv.LibraryTop != nil {
			t.Fatalf("omniscient projection of seat %d lists library contents", p)
		}
	}
	// A seat's own list stays the canonical (name, id) one, not the true order.
	own := seatAfter.Players[seat].Library
	for i := 1; i < len(own); i++ {
		if strings.Compare(own[i-1].Name, own[i].Name) > 0 || (own[i-1].Name == own[i].Name && own[i-1].ID > own[i].ID) {
			t.Fatalf("seat's own library list is not canonically sorted at %d", i)
		}
	}
	if seatAfter.Players[1].Library != nil {
		t.Fatal("seat sees an opponent's library list")
	}
}

func sortedFirst(cvs []CardView) state.ObjID {
	m := cvs[0].ID
	for _, c := range cvs {
		if c.ID < m {
			m = c.ID
		}
	}
	return m
}
