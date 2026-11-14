package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatEachPlayerControllerFirst: gorge's RepeatEach over players
// walks them from the resolving controller (repeatPlayers: AliveFrom), so
// OpRepeatEachPlayer with C&1 binds the controller first even on the
// opponent's turn; without it the order is APNAP.
func TestRepeatEachPlayerControllerFirst(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	sc.s.active = 0
	sc.s.res.e.ctrl = 1
	in := kop.Instr{Op: kop.OpRepeatEachPlayer, Obj: kop.RefEachPlayer, A: kop.Const(0), C: 1}
	sc.s.exec(sc.k, &in)
	if sc.s.g.eachPl != 1 || sc.s.g.repNext != repOne(0) {
		t.Fatalf("C=1: first %d next %d, want the controller (1) then 0", sc.s.g.eachPl, sc.s.g.repNext)
	}
	in.C = 0
	sc.s.exec(sc.k, &in)
	if sc.s.g.eachPl != 0 || sc.s.g.repNext != repOne(1) {
		t.Fatalf("C=0: first %d next %d, want APNAP (0 then 1)", sc.s.g.eachPl, sc.s.g.repNext)
	}
	// The controller is the active player: APNAP still starts with seat 0,
	// so the C=0 swap must not reverse it.
	sc.s.res.e.ctrl = 0
	sc.s.exec(sc.k, &in)
	if sc.s.g.eachPl != 0 || sc.s.g.repNext != repOne(1) {
		t.Fatalf("C=0 ctrl==active: first %d next %d, want APNAP (0 then 1)", sc.s.g.eachPl, sc.s.g.repNext)
	}
}

// repOne is the each-player chain (g.repNext) holding the one seat p still
// to come, in the build's encoding (seatsPack: the seat itself at two seats,
// packed under a count at more).
func repOne(p uint8) uint8 {
	return seatsPack(seatSeq{s: [MaxSeats]uint8{p}, n: 1}, 0)
}

// TestPlayersControllerFirst: every RefEachPlayer consumer (draw, life,
// mill, poison, energy, set-life) walks AliveFrom(controller), also when
// the controller is not the active player.
func TestPlayersControllerFirst(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	sc.s.active = 0
	sc.s.res.e.ctrl = 1
	if q := sc.s.players(sc.k, kop.RefEachPlayer); q.n != 2 || q.s[0] != 1 || q.s[1] != 0 {
		t.Fatalf("ctrl 1 active 0: players %v, want the controller (1) then 0", q.s[:q.n])
	}
	sc.s.res.e.ctrl = 0
	if q := sc.s.players(sc.k, kop.RefEachPlayer); q.n != 2 || q.s[0] != 0 || q.s[1] != 1 {
		t.Fatalf("ctrl 0 active 0: players %v, want 0 then 1", q.s[:q.n])
	}
}
