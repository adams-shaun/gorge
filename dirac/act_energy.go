package dirac

import (
	"unsafe"

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
// coldArrays.exper (genState has no room left): gorge's "Experience" player counter (PutCounter Defined$
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
		c := s.plCtrPtr(q.s[i], int(in.C))
		if c == nil {
			continue // unreachable: tableGainsExper sized the array
		}
		*c = uint8(min(int(*c)+n, 255))
	}
}

// plCtrPtr is player p's counter of player counter kind kind (kop.PlCtr*), or
// nil for experience in a table that never gains any.
func (s *State) plCtrPtr(p uint8, kind int) *uint8 {
	if kind == kop.PlCtrExperience {
		if c := s.experCold(); int(p) < len(c) {
			return &c[p]
		}
		return nil
	}
	return &s.g.energy[p]
}

// experCold is the per-seat experience counters, or nil when the table never
// gains any (coldLayout.exper is 0).
func (s *State) experCold() []uint8 {
	l := s.cold.layout
	if l == nil || l.exper == 0 {
		return nil
	}
	return unsafe.Slice((*uint8)(unsafe.Add(unsafe.Pointer(&s.cold.buf[0]), l.experAt)), l.exper)
}

// plCtr is the value of player p's counter of player counter kind kind.
func (s *State) plCtr(p uint8, kind int) int {
	if c := s.plCtrPtr(p, kind); c != nil {
		return int(*c)
	}
	return 0
}

// tableGainsExper reports whether some ability of t gives experience
// counters (OpGainEnergy with kop.PlCtrExperience); only then does the State
// keep the array (genPrep.anyExper).
func tableGainsExper(t *kop.Table) bool {
	prog := func(a *kop.Ability) bool {
		for i := range a.Prog {
			if in := &a.Prog[i]; in.Op == kop.OpGainEnergy && in.C == kop.PlCtrExperience {
				return true
			}
		}
		return false
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if prog(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if prog(&t.Emblems[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
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
func (s *State) Experience(p uint8) int { return s.plCtr(SeatIx(p), kop.PlCtrExperience) }

// payEnergyN pays n energy for an unless / if-you-do cost (kop.PayEnergy;
// asked only when payable, else decline-only).
//
//go:noinline
func (s *State) payEnergyN(p uint8, n uint8) {
	if p < MaxSeats {
		s.g.energy[p] -= min(s.g.energy[p], n)
	}
}
