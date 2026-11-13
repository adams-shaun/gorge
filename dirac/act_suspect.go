package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the suspected designation (CR 701.60; gorge effects/misc.go
// effAlterAttribute Attributes$ Suspected, Object.Suspected). The designation
// is the arena slot bit mechSuspected (arena.go):
// ExtSuspect sets or clears it, and it
//   - gives the permanent menace as a base keyword (gorge BaseKeywords
//     appends it ahead of the layer walk, where the cloak's status ward
//     lands, so a lose-all effect removes it too; kwGen, kw6),
//   - stops it blocking (canBlock; gorge combat/block.go),
//   - ends when any control change moves it (ctrlMove, the end-of-turn revert
//     of a GainControl included; gorge events apply_player.go),
//   - ends when it leaves the battlefield: enterGen's arenaClrSlot clears the
//     slot's bit as the next permanent takes it. No reader runs in between: a
//     vacated slot is outside s.occ, which every battlefield walk, filter match
//     and projection iterates, and the one reader off the mask (canBlock, kwGen)
//     takes an occupied slot.
// Cold and out of line: a table with no ExtSuspect takes none of these paths
// (genPrep.anySuspect).

// DebugSuspectCanBlock makes canBlock ignore the designation. The lockstep
// mutation test flips it and requires a divergence.
var DebugSuspectCanBlock bool

// prepSuspect sets anySuspect: the table gives mechSuspected an arena mask.
func prepSuspect(g *genPrep) { g.anySuspect = g.ix[mechSuspected] != arenaNone }

// suspected reports whether the permanent at sl is suspected.
func (s *State) suspected(k *prep, sl Slot) bool { return s.mSl(k, mechSuspected, sl) }

// suspectKw is the keywords the designation gives the permanent at sl.
func (s *State) suspectKw(k *prep, sl Slot) kop.KwMask {
	if s.mSl(k, mechSuspected, sl) {
		return kop.KwMenace
	}
	return 0
}

// suspectNoBlock reports whether the designation stops blocker b blocking.
func (s *State) suspectNoBlock(k *prep, b Slot) bool {
	return !DebugSuspectCanBlock && s.mSl(k, mechSuspected, b)
}

// suspectExt is ExtSuspect: Obj, or every battlefield permanent matching F
// with Obj RefNone (gorge effAlterAttribute over Defined$: battlefield
// objects only), becomes suspected, or with C bit0 no longer suspected.
func (s *State) suspectExt(k *prep, in *kop.Instr) {
	set := in.C&1 == 0 // seat-ok: an instruction flag bit
	if in.Obj == kop.RefNone {
		var ord [MaxPerm]Slot
		n := s.sweepHits(k, in.F, &ord)
		for _, sl := range ord[:n] {
			s.suspectOne(k, sl, set)
		}
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		s.suspectOne(k, sl, set)
	}
}

func (s *State) suspectOne(k *prep, sl Slot, set bool) {
	if s.mSl(k, mechSuspected, sl) == set {
		return
	}
	if set && s.cantSuspect(k, sl) {
		return // Airtight Alibi (gorge suppressSuspectedEvent)
	}
	if set {
		s.mSlSet(k, mechSuspected, sl)
	} else {
		s.mSlClr(k, mechSuspected, sl)
	}
	s.touchSl(sl)
}

// suspectWouldChange is the OpMayAsk guard of an Optional$ suspect (gorge
// alterAttributeWouldChange): the election is posed only when the next
// ExtSuspect would flip some object. A StatCantBeSuspected static is not read
// here: gorge reads it only at the emit choke point, so the ask is still posed
// and suspectOne then swallows the answer.
func (s *State) suspectWouldChange(k *prep, next *kop.Instr) bool {
	set := next.C&1 == 0 // seat-ok: an instruction flag bit
	if next.Obj == kop.RefNone {
		var ord [MaxPerm]Slot
		n := s.sweepHits(k, next.F, &ord)
		for _, sl := range ord[:n] {
			if s.mSl(k, mechSuspected, sl) != set {
				return true
			}
		}
		return false
	}
	sl := s.objRef(k, next.Obj)
	return sl != NoSlot && s.mSl(k, mechSuspected, sl) != set
}

// DebugCantSuspectOff is a mutation switch: a StatCantBeSuspected static
// forbids nothing, so the creature Airtight Alibi enchants becomes
// suspected and the lockstep repro diverges.
var DebugCantSuspectOff bool

// cantSuspect reports whether a StatCantBeSuspected static (S:Mode$
// CantBeSuspected, gorge rules/cantbesuspected.go) forbids the permanent at
// sl from becoming suspected (CR 701.60). Only the set direction reads it:
// a designation already held stays (gorge gates the event only).
func (s *State) cantSuspect(k *prep, sl Slot) bool {
	return k.gen != nil && k.gen.anyCantSuspect && !DebugCantSuspectOff && s.staticFlagGen(k, kop.StatCantBeSuspected, sl)
}
