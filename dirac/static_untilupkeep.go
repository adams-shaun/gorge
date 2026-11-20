package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// useUpkeepOv is the table flag standing in for a genPrep bool (genneed.go):
// a table with an UntilYourNextUpkeep override gets a genPrep, and upkeepDrop
// reads the bit, so a pool with no such record takes neither the upkeep scan
// nor the keyword restore.
var useUpkeepOv = newGenUse(tableHasUpkeepOv)

// tableHasUpkeepOv reports whether any card program or emblem program applies
// an OpOverride / OpOverrideAll lasting until its controller's next upkeep.
func tableHasUpkeepOv(t *kop.Table) bool {
	has := func(prog []kop.Instr) bool {
		for i := range prog {
			in := &prog[i]
			if (in.Op == kop.OpOverride || in.Op == kop.OpOverrideAll) && in.A.K == kop.VConst && in.A.N == kop.DurUntilYourNextUpkeep {
				return true
			}
		}
		return false
	}
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if has(t.Cards[i].Abilities[j].Prog) {
				return true
			}
		}
	}
	for i := range t.Emblems {
		for j := range t.Emblems[i].Abilities {
			if has(t.Emblems[i].Abilities[j].Prog) {
				return true
			}
		}
	}
	return false
}

// upkeepDrop ends the UntilYourNextUpkeep records as their controller's upkeep
// step begins, before firePhase puts that step's triggers on the stack: gorge
// drops them in setStep before the StepChange emit
// (layers_effect.go: dropUntilYourNextUpkeep). useUpkeepOv gates it to a pool
// that holds at least one such record; the copy-exception keyword restore runs
// after the pool changes the way cleanupGen's does.
func (s *State) upkeepDrop(k *prep) {
	if k.gen != nil && k.gen.uses.has(useUpkeepOv) {
		s.copyExcKwRestore(k, s.dropUpkeepOv(k))
	}
}

// dropUpkeepOv ends the override records lasting until their controller's next
// upkeep begins (Duration$ UntilYourNextUpkeep, CR 611.2b; Erhnam Djinn) as
// that upkeep step is entered.
//
// The records ride the ovNT bits (static_control.go), but the cleanup drop
// skips the rows whose kop.Override.UntilUpkeep is set, so their controller's
// turn beginning is not their end. The end point follows the controller stamped
// in ovNTWho at application, not the target's controller.
func (s *State) dropUpkeepOv(k *prep) (ended slotMask) {
	n := 0
	s.fxM = slotMask{}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.kind == fxOverride && r.r.fl&ovNT != 0 &&
			k.t.Overrides[r.r.idx-1].UntilUpkeep &&
			(r.r.fl&ovNTWho)>>ovNTWhoShift == s.active {
			s.touchSl(r.sl)
			ended = ended.with(r.sl)
			continue
		}
		s.g.fx[n] = *r
		s.fxM.set(r.sl)
		n++
	}
	for i := n; i < int(s.g.nfx); i++ {
		s.g.fx[i] = fxRec{}
	}
	s.g.nfx = uint8(n)
	return ended
}
