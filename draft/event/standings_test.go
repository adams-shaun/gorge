package event_test

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// resultOf builds one SeriesResult from a winner seat (-1 draw) over one
// game.
func resultOf(round, a, b int, winner int) event.SeriesResult {
	r := event.SeriesResult{Round: round, A: a, B: b}
	switch winner {
	case -1:
		r.Games = []event.GameOutcome{event.Draw}
	default:
		if winner == a {
			r.Score = [2]int{1, 0}
			r.Games = []event.GameOutcome{event.AWon}
		} else {
			r.Score = [2]int{0, 1}
			r.Games = []event.GameOutcome{event.BWon}
		}
	}
	return r
}

// rowFor returns the standings row of one seat.
func rowFor(t *testing.T, st []event.Standing, seat int) event.Standing {
	t.Helper()
	if len(st) != 4 {
		t.Fatalf("standings has %d rows, want 4", len(st))
	}
	for _, row := range st {
		if row.Seat == seat {
			return row
		}
	}
	t.Fatalf("seat %d missing from the standings", seat)
	return event.Standing{}
}

// TestStandingsOMWTieBreak: a two-way points tie broken by OMW%. After four
// series, seats 1 and 3 both sit on 3 points; seat 3's opponents are worth
// 1.0 and 0.5 (OMW 0.75) while seat 1's are worth 0.33 and 0.5 (OMW 0.415),
// so seat 3 ranks above seat 1 on OMW%.
func TestStandingsOMWTieBreak(t *testing.T) {
	rs := []event.SeriesResult{
		resultOf(1, 0, 3, 0), // 0 beats 3
		resultOf(1, 1, 2, 1), // 1 beats 2
		resultOf(2, 0, 2, 0), // 0 beats 2
		resultOf(2, 1, 3, 3), // 3 beats 1
	}
	st := event.ComputeStandings(rs)

	s1, s3 := rowFor(t, st, 1), rowFor(t, st, 3)
	if s1.Points != 3 || s3.Points != 3 {
		t.Fatalf("the tie precondition does not hold: seats 1 and 3 have %d and %d points, want 3 and 3",
			s1.Points, s3.Points)
	}
	pos1, pos3 := -1, -1
	for i, row := range st {
		switch row.Seat {
		case 1:
			pos1 = i
		case 3:
			pos3 = i
		}
	}
	if pos3 > pos1 {
		t.Errorf("seat 3 (OMW %g) sits below seat 1 (OMW %g): the tie was not broken by OMW%%",
			s3.OMW, s1.OMW)
	}
	if s3.OMW <= s1.OMW {
		t.Errorf("seat 3's OMW %g does not beat seat 1's %g", s3.OMW, s1.OMW)
	}
	if rowFor(t, st, 2).MW < 0.33-1e-9 || rowFor(t, st, 2).MW > 0.33+1e-9 {
		t.Errorf("seat 2's MW is %g, want the 0.33 floor for a winless seat", rowFor(t, st, 2).MW)
	}
}

// TestStandingsFloorChangesOrder: a mid-event table where the 0.33 floor
// changes the order. Seats 0 and 3 are tied on 1 point; seat 3's raw MW is
// 1/6 = 0.167 (two series) and seat 0's is 1/9 = 0.111 (three series), so
// the raw order is 3 above 0 — but both floor to 0.33, and seat 0's OMW%
// (0.33, 1.0 and 0.667 opponents) beats seat 3's (0.33 and 0.667), putting
// seat 0 above seat 3: the reverse of the raw order.
func TestStandingsFloorChangesOrder(t *testing.T) {
	rs := []event.SeriesResult{
		resultOf(1, 0, 3, -1), // 0 draws 3
		resultOf(1, 1, 2, 2),  // 2 beats 1
		resultOf(2, 1, 3, 1),  // 1 beats 3
		resultOf(2, 0, 2, 2),  // 2 beats 0
		resultOf(3, 0, 1, 1),  // 1 beats 0; 3's round-3 series is still pending
	}
	st := event.ComputeStandings(rs)

	s0, s3 := rowFor(t, st, 0), rowFor(t, st, 3)
	if s0.Points != 1 || s3.Points != 1 {
		t.Fatalf("precondition: seat 0 has %d points and seat 3 has %d, want 1 and 1", s0.Points, s3.Points)
	}
	for _, row := range []event.Standing{s0, s3} {
		if row.MW < 0.33-1e-9 || row.MW > 0.33+1e-9 {
			t.Fatalf("precondition: seat %d's MW is %g, want the 0.33 floor", row.Seat, row.MW)
		}
	}
	if s0.OMW <= s3.OMW {
		t.Fatalf("precondition: seat 0's OMW %g should beat seat 3's %g for the floor to change the order",
			s0.OMW, s3.OMW)
	}
	for _, row := range []event.Standing{s0, s3} {
		if row.GW < 0.33-1e-9 || row.GW > 0.33+1e-9 {
			t.Fatalf("precondition: seat %d's GW is %g, want the 0.33 floor — GW ties too, so only OMW can reverse the raw order", row.Seat, row.GW)
		}
	}
	pos0, pos3 := -1, -1
	for i, row := range st {
		switch row.Seat {
		case 0:
			pos0 = i
		case 3:
			pos3 = i
		}
	}
	if pos0 > pos3 {
		t.Errorf("the 0.33 floor did not change the order: seat 0 (raw MW 0.111) sits below seat 3 (raw MW 0.167) at positions %d and %d",
			pos0, pos3)
	}
}

// TestStandingsFullEvent pins the full scripted event's table.
func TestStandingsFullEvent(t *testing.T) {
	e := newScriptedEvent(t)
	st := e.Standings()
	for i, want := range []struct {
		seat, points int
	}{
		{0, 7}, {2, 6}, {3, 3}, {1, 1},
	} {
		if st[i].Seat != want.seat || st[i].Points != want.points {
			t.Errorf("row %d is seat %d with %d points, want seat %d with %d",
				i, st[i].Seat, st[i].Points, want.seat, want.points)
		}
	}
}
