package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// rollDie is OpRollDie (gorge effRollDice, one die): one draw of the
// engine rng (gorge h.Rand(sides) = rng.IntN; the kernel's rng is gorge's
// PCG, so the draw order matches), result = draw + 1 + modifier, kept as
// the resolution's roll (ExRollResult). gorge reads sides <= 0 as 6.
//
//go:noinline
func (s *State) rollDie(k *prep, in *kop.Instr) {
	sides := s.value(k, in.A)
	if sides <= 0 {
		sides = 6
	}
	if in.C == kop.RollDieCoin {
		// gorge effects.FlipCoinWin: a forced flip is heads with no rng draw
		// (the FlipCoinMod static reads the controller's first flip this
		// turn); every flip, forced or drawn, marks the flipper's turn bit.
		p := s.res.e.ctrl
		if s.flipForced(k, p) {
			s.g.roll = 1
			if k.gen != nil && k.gen.anyTurnF {
				s.setTurnFlip(p)
			}
			return
		}
		s.g.roll = uint8(max(0, min(s.rng.IntN(sides)+1+s.value(k, in.B), 255)))
		if k.gen != nil && k.gen.anyTurnF {
			s.setTurnFlip(p)
		}
		return
	}
	r := s.rng.IntN(sides) + 1 + s.value(k, in.B)
	s.g.roll = uint8(max(0, min(r, 255)))
}
