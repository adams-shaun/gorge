package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Pump Duration$ UntilEndOfCombat as an override record whose static row is
// marked kop.Override.EndCombat. The record is applied with no duration bits
// (kop.DurPermanent), so the end-of-turn sweep leaves it alone; the sweeps
// here end it:
//
//   - endCombatOv, on the transition that leaves the end-of-combat step
//     (turn.go advance; gorge expireUntilEndOfCombat, CR 511.2). A second
//     combat phase starts without the record, and a record made after that
//     step lasts to cleanup.
//   - dropEndCombatOv, at cleanup (turn_gen.go cleanupGen): when combat was
//     skipped the step never ran, and gorge's EndOfTurnCleanup
//     untilEndOfCombat backstop ends the record with the turn.

// useEndCombatOv is the table gate: some override row ends with the combat. It
// is a genPrep.uses bit, not a new genPrep field (layout_pins_test.go,
// TestNoNewFixedStateFields).
var useEndCombatOv = newGenUse(func(t *kop.Table) bool {
	for i := range t.Overrides {
		if t.Overrides[i].EndCombat {
			return true
		}
	}
	return false
})

// DebugNoPumpEndCombat (the mutation): an EndCombat record is not dropped on
// the way out of the end-of-combat step (the cleanup backstop still runs).
var DebugNoPumpEndCombat bool

// endCombatOv drops the EndCombat override records as the end-of-combat step
// ends.
func (s *State) endCombatOv(k *prep) {
	if DebugNoPumpEndCombat {
		return
	}
	s.copyExcKwRestore(k, s.dropEndCombatOv(k))
}

// dropEndCombatOv drops every EndCombat override record and returns the
// slots that lost one.
func (s *State) dropEndCombatOv(k *prep) (ended slotMask) {
	if !s.fxM.any() {
		return
	}
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.kind == fxOverride && k.t.Overrides[r.r.idx-1].EndCombat {
			ended = ended.with(r.sl)
		}
	}
	if ended.any() {
		s.dropFx(func(i int) bool {
			r := &s.g.fx[i]
			return r.kind == fxOverride && k.t.Overrides[r.r.idx-1].EndCombat
		})
	}
	return
}
