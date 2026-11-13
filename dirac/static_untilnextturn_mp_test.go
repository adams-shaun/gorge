//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestOvNextTurnFourSeatController: an UntilYourNextTurn override record ends
// at the turn following its controller's (CR 613, Duration$
// UntilYourNextTurn), so dropNextTurnOv must compare the record's controller
// (ovNTWho, seats_mp.go) with nextActive, seat by seat. At four seats one
// bit of controller could not name seats 2 and 3, so this is where the
// comparison goes wrong (four-seat Undercity's Throne room dropped its
// hexproof before seat 0's turn). Precondition: nextActive() is the seat the
// assertion keys on — s.active is set and no seat has extra turns pending.
func TestOvNextTurnFourSeatController(t *testing.T) {
	const n = 4
	var decks [MaxSeats][]CardID
	for p := 0; p < n; p++ {
		decks[p] = vanilla.Decks[p%len(vanilla.Decks)]
	}
	s := new(State)
	s.InitN(vanilla.Table, decks, n, 7, Config{Mulligans: 1})
	if s.nSeats() != n {
		t.Fatalf("nSeats %d", s.nSeats())
	}
	for _, active := range [][2]uint8{{0, 0}, {1, 1}, {2, 2}, {3, 3}} {
		st := &s.pl[active[0]]
		if st.extraTurns != 0 {
			t.Fatalf("seat %d has %d extra turns; nextActive would not follow it", active[0], st.extraTurns)
		}
	}
	for _, tc := range [][2]uint8{{2, 3}, {2, 1}, {3, 0}, {3, 2}, {0, 3}, {1, 0}} {
		ctrl, active := tc[0], tc[1]
		s.res.e.ctrl = ctrl
		s.active = active
		s.g.fx[0] = fxRec{kind: fxOverride, r: ovRec{idx: 1, fl: s.ovDurFl(kop.DurUntilYourNextTurn)}}
		s.g.nfx = 1
		want := s.nextActive() == ctrl
		ended := s.dropNextTurnOv()
		if got := ended.has(0); got != want {
			t.Fatalf("ctrl=%d active=%d next=%d: dropped=%v want=%v", ctrl, active, s.nextActive(), got, want)
		}
	}
}
