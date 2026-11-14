//go:build dirac_mp

package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestOppStateReadsEverySeat: the opponent-state reads sum or count over
// every opponent (gorge LifeOppsLostThisTurn, PlayerCountRegisteredOpponents),
// not the controller's one other seat, and an expression's opponent ref is
// the first of gorge's Opponent list.
func TestOppStateReadsEverySeat(t *testing.T) {
	for gone := -1; gone < 4; gone++ {
		e, sc, _ := oppScen(t, gone)
		s, k := sc.s, sc.k
		s.pl[2].lifeLost, s.pl[3].lifeLost = 3, 4
		s.g.cdLeg = 1 << 3
		for ctrl := uint8(0); ctrl < 4; ctrl++ {
			if int(ctrl) == gone {
				continue
			}
			want, cnt, leg := 0, 0, 0
			for q := uint8(0); q < 4; q++ {
				if q != ctrl {
					want += int(s.pl[q].lifeLost)
					if s.pl[q].lost {
						continue
					}
					if s.pl[q].lifeLost > 0 {
						cnt++
					}
					if s.g.cdLeg>>q&1 != 0 {
						leg++
					}
				}
			}
			if got := s.oppsLifeLost(ctrl); got != want {
				t.Fatalf("gone%d ctrl%d: opponents lost %d, want %d", gone, ctrl, got, want)
			}
			if got := s.oppsLostLife(ctrl, 0); got != cnt {
				t.Fatalf("gone%d ctrl%d: %d opponents lost life, want %d", gone, ctrl, got, cnt)
			}
			if got := s.oppsLostLife(ctrl, 1); got != leg {
				t.Fatalf("gone%d ctrl%d: %d opponents hit by a legend, want %d", gone, ctrl, got, leg)
			}
			opps := aliveFromGorge(e, int(ctrl), int(ctrl))
			if got := s.refPlayer(k, kop.RefOpponent, evalCtx{src: NoObj, ctrl: ctrl}); got != opps[0] {
				t.Fatalf("gone%d ctrl%d: opponent ref %d, gorge %d", gone, ctrl, got, opps[0])
			}
		}
	}
}

// TestNextTurnPlayerSkipsTheGone: the next turn passes to the next seat still
// in the game (gorge NextAlive), an extra turn first.
func TestNextTurnPlayerSkipsTheGone(t *testing.T) {
	for gone := -1; gone < 4; gone++ {
		e, sc, _ := oppScen(t, gone)
		s := sc.s
		for a := uint8(0); a < 4; a++ {
			if int(a) == gone {
				continue
			}
			s.active = a
			want := uint8(e.G.NextAlive(state.PlayerID(a)))
			if got := s.NextTurnPlayer(); got != want {
				t.Fatalf("gone%d active%d: next %d, gorge %d", gone, a, got, want)
			}
			if got := s.nextActive(); got != want {
				t.Fatalf("gone%d active%d: nextActive %d, gorge %d", gone, a, got, want)
			}
		}
	}
}

// TestSpeedComparesEverySeat: "only a player whose speed exceeds every other
// player's" reads each other seat.
func TestSpeedComparesEverySeat(t *testing.T) {
	_, sc, _ := oppScen(t, -1)
	s := sc.s
	if s.speedMatched(0, 1) {
		t.Fatal("no seat has speed")
	}
	if !s.speedMatched(0, 0) {
		t.Fatal("speed 0 matches speed 0")
	}
}

// TestControlRecordHoldsFourSeats: a layer-2 control record names seats 2
// and 3 (two bits a seat in r.fl).
func TestControlRecordHoldsFourSeats(t *testing.T) {
	for to := uint8(0); to < 4; to++ {
		for prev := uint8(0); prev < 4; prev++ {
			var r fxRec
			r.kind = fxControl
			r.r.fl = to | 3<<4
			r.setPrev(prev)
			if r.cTo() != to || r.cPrev() != prev || r.cYou() != 3 {
				t.Fatalf("to %d prev %d: read %d %d %d", to, prev, r.cTo(), r.cPrev(), r.cYou())
			}
		}
	}
}

// TestPerSeatBitsFitFourSeats: the packed per-seat flags hold seat 3 apart
// from every other seat's.
func TestPerSeatBitsFitFourSeats(t *testing.T) {
	_, sc, _ := oppScen(t, -1)
	s := sc.s
	s.unspentRedOn(3)
	if s.pmOn != pmUnspentR<<3 || s.pmOn&(1<<MaxSeats-1) != 0 {
		t.Fatalf("unspent red at seat 3: pmOn %08b", s.pmOn)
	}
	var c czallPend
	c.add(3, []Obj{1}, []kop.Zone{kop.ZoneGraveyard})
	if c.n[3] != 1 {
		t.Fatal("czallPend lost seat 3's card")
	}
}

// TestOppStateTwoSeatMarks: a table whose speed, battle, goad or skip-untap
// state packs two seats refuses a larger game.
func TestOppStateTwoSeatMarks(t *testing.T) {
	var tb kop.Table
	tb.Cards = []kop.CardDef{{Types: kop.TBattle}}
	var g genPrep
	prepTwoSeat(&tb, &g)
	if !g.twoSeatOnly {
		t.Fatal("a battle's protector is the one opponent: two seats only")
	}
}
