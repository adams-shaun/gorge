//go:build dirac_mp

package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEachOpponentDiscardsN: "each opponent discards" at four seats is
// gorge's effDiscard over Defined$ Opponent, AliveFrom(controller) without
// the controller, a seat out of the game dropped: every opponent discards,
// not only the first.
func TestEachOpponentDiscardsN(t *testing.T) {
	for gone := -1; gone < 4; gone++ {
		for ctrl := 0; ctrl < 4; ctrl++ {
			if ctrl == gone {
				continue
			}
			e, sc, id := oppScen(t, gone)
			s, k := sc.s, sc.k
			s.res.e.ctrl = uint8(ctrl)
			opps := fmt.Sprint(aliveFromGorge(e, ctrl, ctrl))
			if got := seatsOf(s.oppsFrom(uint8(ctrl))); got != opps {
				t.Fatalf("gone%d ctrl%d: oppsFrom %s, gorge %s", gone, ctrl, got, opps)
			}
			for _, f := range []func() seatSeq{
				func() seatSeq { return s.players(k, kop.RefEachOpponent) },
				func() seatSeq { return s.eachPlayers(k, kop.RefEachOpponent) },
			} {
				if got := seatsOf(f()); got != opps {
					t.Fatalf("gone%d ctrl%d: each opponent %s, gorge %s", gone, ctrl, got, opps)
				}
			}
			for p := uint8(0); p < 4; p++ {
				sc.hand(id["Plains"], p)
			}
			in := &kop.Instr{Op: kop.OpDiscard, Obj: kop.RefEachOpponent, A: kop.Const(1), C: kop.ChooseSelf}
			if !s.exec(k, in) {
				t.Fatalf("gone%d ctrl%d: a whole-hand discard posed an ask", gone, ctrl)
			}
			for p := 0; p < 4; p++ {
				want := 0
				if p == ctrl || p == gone {
					want = 1
				}
				if int(s.pl[p].handN) != want {
					t.Fatalf("gone%d ctrl%d: seat %d holds %d cards, want %d", gone, ctrl, p, s.pl[p].handN, want)
				}
			}
		}
	}
}

// TestDamagedPermanentSeat: a damage trigger's player for a permanent
// recipient is that permanent's controller at more than two seats, and the
// damaged seat for a player recipient.
func TestDamagedPermanentSeat(t *testing.T) {
	_, sc, id := oppScen(t, -1)
	s := sc.s
	for p := uint8(0); p < 4; p++ {
		x := sc.perm(id["Plains"], p)
		if got := s.dmgSeat(uint8(x), 0); got != p {
			t.Fatalf("damage to seat %d's permanent names seat %d", p, got)
		}
		if got := s.dmgSeat(PlayerTarget0+p, 1); got != p {
			t.Fatalf("damage to seat %d names seat %d", p, got)
		}
	}
}
