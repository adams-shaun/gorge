package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Kernel arms, cold and out
// of line.

// Energy (CR 122.1, 107.14): a player counter, kept per player in
// genState.energy (2 B in the padding after tli: State does not grow).
// gorge keeps it as the player's "ENERGY" counter (PutCounter Defined$
// <player> | CounterType$ ENERGY) and pays PayEnergy<N> from it
// (rules/pay EnergyPayable). The kernel saturates at 255 (gorge's int32
// never does; no corpus line reaches it).

// gainEnergy is OpGainEnergy: player(s) Obj get A energy counters.
//
//go:noinline
func (s *State) gainEnergy(k *prep, in *kop.Instr) {
	p1, p2 := s.players(k, in.Obj)
	n := s.value(k, in.A)
	if n <= 0 {
		return
	}
	for _, p := range [2]uint8{p1, p2} {
		if p > 1 {
			continue
		}
		s.g.energy[p] = uint8(min(int(s.g.energy[p])+n, 255))
	}
}

// energyPayable: p has the N energy a CostPayEnergy cost needs.
//
//go:noinline
func (s *State) energyPayable(p uint8, c *kop.Cost) bool {
	return p <= 1 && s.g.energy[p] >= c.N
}

// payEnergy pays a CostPayEnergy cost.
//
//go:noinline
func (s *State) payEnergy(p uint8, c *kop.Cost) {
	if p <= 1 {
		s.g.energy[p] -= min(s.g.energy[p], c.N)
	}
}

// Energy is player p's energy counter count (oracle / tests).
func (s *State) Energy(p uint8) int { return int(s.g.energy[SeatIx(p)]) }

// payEnergyN pays n energy for an unless / if-you-do cost (kop.PayEnergy;
// asked only when payable, else decline-only).
//
//go:noinline
func (s *State) payEnergyN(p uint8, n uint8) {
	if p <= 1 {
		s.g.energy[p] -= min(s.g.energy[p], n)
	}
}
