package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugKiraMutate is the test-only mutation switch of TestKiraMutation: 1
// enters tapped but not attacking, 2 drops the IsPresent gate of an
// other-enter program.
var DebugKiraMutate uint8

// enterAttackingRepl is ExtEnterAttacking: the permanent at sl, entering under
// a replacement program (it is not yet in occ, enterGen cleared it), enters
// attacking (gorge ChangeZone Attacking$ True folded into the entry; CR 508.4:
// never declared, so no Attacks trigger). Same window and defender rule as
// enterAttacking (zone_changezoneparams.go): the controller's own declare
// attackers..end of combat, the opponent as the one CR 508.4 defender. Two or
// more defenders would be gorge's defender ask, which this route cannot pose
// mid-entry: it stalls LOUD (StallTokAtkAsk) rather than pick one.
//
//go:noinline
func (s *State) enterAttackingRepl(k *prep, sl Slot, ctrl uint8) {
	if DebugKiraMutate == 1 {
		return
	}
	if sl == NoSlot || ctrl != s.active || s.step < kop.StepAttackers || s.step > kop.StepEndCombat {
		return
	}
	var buf [MaxOptions]TCode
	n := s.enteringDefenderCands(k, ctrl, &buf)
	switch {
	case n == 0:
		return // no CR 508.4 defender: it enters but does not attack
	case n > 1:
		s.capFull(StallTokAtkAsk)
		return
	}
	s.setEnterAttacker(k, sl, buf[0])
}
