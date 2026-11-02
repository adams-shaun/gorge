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
	r := s.rng.IntN(sides) + 1 + s.value(k, in.B)
	s.g.roll = uint8(max(0, min(r, 255)))
}
