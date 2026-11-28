package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// "Any opponent may" (UnlessPayer$ Opponent, kgen obj=EachOpp; Vexing Devil)
// at four seats asks every living opponent in seat order from player 0, the
// controller skipped, until one pays: gorge unlessPayerTargetsFor's Opponent
// arm sorted by AliveFrom(0) rank (sortTargets), Forge's game player order.
// Before the walk, the kernel asked only oneOpp (the seat after the
// controller).

// eachOppAsks resolves an unless-switched PayLife 4 over EachOpp controlled
// by ctrl at four seats, the seat lost (if >= 0) out of the game, declining
// every ask; it returns who was asked.
func eachOppAsks(t *testing.T, ctrl uint8, lost int) []uint8 {
	t.Helper()
	prog := []kop.Instr{
		{Op: kop.OpUnlessSwitched, Obj: kop.RefEachOpponent, A: kop.Const(4), C: kop.PayLife | 1<<4},
		{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)},
	}
	sc, fc := cmd5HostScen(t, prog)
	if !sc.k.eachPay {
		t.Fatal("prep.eachPay not set for an EachOpp pay op")
	}
	s := sc.s
	s.g.nseat = 4
	for p := 0; p < 4; p++ {
		s.pl[p].life = 20
	}
	if lost >= 0 {
		s.pl[lost].lost = true
	}
	o := sc.perm(fc, ctrl)
	e := stackEntry{kind: seAbility, obj: o, ctrl: ctrl, card: fc, ab: uint8(len(sc.k.t.Cards[fc].Abilities) - 1), trigObj: NoObj}
	s.push(e)
	s.res.e = e
	s.res.pc = 0
	s.runProg(sc.k)
	var asked []uint8
	for i := 0; i < 8; i++ {
		d := s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenPayMode {
			break
		}
		asked = append(asked, d.Player)
		s.Apply(MakeAction(AkChoose, 0, 0, 0)) // decline
	}
	return asked
}

func TestEachOpponentPayerSeatOrder(t *testing.T) {
	for _, c := range []struct {
		name string
		ctrl uint8
		lost int
		want []uint8
	}{
		{"controller p2", 2, -1, []uint8{0, 1, 3}},
		{"controller p0", 0, -1, []uint8{1, 2, 3}},
		{"controller p3", 3, -1, []uint8{0, 1, 2}},
		{"controller p2, p0 lost", 2, 0, []uint8{1, 3}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := eachOppAsks(t, c.ctrl, c.lost)
			if len(got) != len(c.want) {
				t.Fatalf("asked %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("asked %v, want %v", got, c.want)
				}
			}
		})
	}
}
