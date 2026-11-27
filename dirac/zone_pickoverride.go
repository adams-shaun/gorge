package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// PickOverrideStats counts the picks whose entry applied a StaticEffect$
// override (the Path of the Schemer's "it's an artifact in addition"). The
// lockstep test asserts it is non-zero, so a green game is not one where the
// new shape never fired.
type PickOverrideStats struct{ Applied int64 }

// DebugPickOverrideStats, when set, receives the pick-override count.
var DebugPickOverrideStats *PickOverrideStats

// DebugPickOverrideOff is the mutation switch: a picked card enters without
// its following OvEnterWith override, so the type-changing read is ignored.
var DebugPickOverrideOff bool

// pickOverrideArm reports whether the resolving instruction is an OpChooseMove
// whose entry takes the OvEnterWith override emitted right after it (kgen's
// graveyard-pick StaticEffect$ rider, lower_zone.go). It arms the entrant's
// override for enterGen, which applies it before the as-enters replacements
// (CR 614.12; gorge applyStaticEffect ahead of the entry event), exactly as a
// known-origin Return's rider does (zone_assimilate.go). Only a one-card pick
// is armed: the rider is one instruction and the hidden-pick lowering refuses
// a wider pick beside a StaticEffect$.
func (s *State) pickOverrideArm(k *prep, in *kop.Instr, z kop.Zone) bool {
	if DebugPickOverrideOff || z != kop.ZoneBattlefield || in == nil || in.Op != kop.OpChooseMove || s.g.nlook != 1 {
		return false
	}
	prog := s.prog(k)
	if int(s.res.pc)+1 >= len(prog) {
		return false
	}
	if nx := &prog[s.res.pc+1]; nx.Op != kop.OpOverride || nx.B.N != kop.OvEnterWith {
		return false
	}
	s.g.ewPend = true
	return true
}

// pickOverrideSettle closes an armed pick entry: enterGen applied the override
// as the permanent entered, and the following OpOverride is spent either way
// (enterWithSettle).
func (s *State) pickOverrideSettle(k *prep, sl Slot, armed bool) {
	if !armed {
		return
	}
	s.enterWithSettle(k, sl, true)
	if DebugPickOverrideStats != nil {
		DebugPickOverrideStats.Applied++
	}
}
