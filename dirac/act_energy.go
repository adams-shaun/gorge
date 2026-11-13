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

// Experience (CR 122.1) is the other player counter the kernel keeps, in
// genState.exper: gorge's "Experience" player counter (PutCounter Defined$
// You | CounterType$ Experience), read by Count$YourCountersExperience. Like
// energy it saturates at 255.

// gainEnergy is OpGainEnergy: player(s) Obj get A counters of player counter
// kind C (kop.PlCtrEnergy or kop.PlCtrExperience).
//
//go:noinline
func (s *State) gainEnergy(k *prep, in *kop.Instr) {
	q := s.players(k, in.Obj)
	n := s.value(k, in.A)
	if n <= 0 {
		return
	}
	for i := uint8(0); i < q.n; i++ {
		c := s.plCtr(q.s[i], int(in.C))
		*c = uint8(min(int(*c)+n, 255))
	}
}

// plCtr is player p's counter of player counter kind kind (kop.PlCtr*).
func (s *State) plCtr(p uint8, kind int) *uint8 {
	if kind == kop.PlCtrExperience {
		return &s.g.exper[p]
	}
	return &s.g.energy[p]
}

// energyPayable: p has the N energy a CostPayEnergy cost needs.
//
//go:noinline
func (s *State) energyPayable(p uint8, c *kop.Cost) bool {
	return p < MaxSeats && s.g.energy[p] >= c.N
}

// payEnergy pays a CostPayEnergy cost.
//
//go:noinline
func (s *State) payEnergy(p uint8, c *kop.Cost) {
	if p < MaxSeats {
		s.g.energy[p] -= min(s.g.energy[p], c.N)
	}
}

// Energy is player p's energy counter count (oracle / tests).
func (s *State) Energy(p uint8) int { return int(s.g.energy[SeatIx(p)]) }

// Experience is player p's experience counter count (oracle / tests).
func (s *State) Experience(p uint8) int { return int(s.g.exper[SeatIx(p)]) }

// payEnergyN pays n energy for an unless / if-you-do cost (kop.PayEnergy;
// asked only when payable, else decline-only).
//
//go:noinline
func (s *State) payEnergyN(p uint8, n uint8) {
	if p < MaxSeats {
		s.g.energy[p] -= min(s.g.energy[p], n)
	}
}
