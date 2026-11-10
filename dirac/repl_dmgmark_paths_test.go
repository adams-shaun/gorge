package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// markedPathState is a state with seat 0's mark on seat 1 and, when doubler,
// a permanent of seat 0 whose card carries rbDblDmg (card 1), in a table with
// the mark's gates open.
func markedPathState(doubler bool) (*State, *prep) {
	s := &State{}
	s.g.nseat = MaxSeats
	g := &genPrep{anyDmgMark: true, anyRepl: rbDblDmg, replBits: []uint32{0, rbDblDmg}}
	k := &prep{gen: g}
	s.res.e.ctrl, s.res.e.trigPl = 0, 1
	s.dmgMarkAdd(nil, &kop.Instr{Obj: kop.RefTriggerPlayer})
	if doubler {
		s.perms[0].card = 1
		s.perms[0].ctrl = 0
		s.statM = s.statM.with(0)
		s.ctrlM[0] = s.ctrlM[0].with(0)
	}
	return s, k
}

// TestDmgMarkSweepGuard: the sweep guard counts the mark as a replacement, so
// a mark beside one per-permanent replacement (or a second mark) is the
// stalled CR 616.1 order ask; a lone mark is not.
func TestDmgMarkSweepGuard(t *testing.T) {
	s, k := markedPathState(false)
	if s.sweepReplCompete(k, NoSlot) {
		t.Fatal("a lone mark competes in the sweep guard")
	}
	s.res.e.ctrl = 1
	s.dmgMarkAdd(nil, &kop.Instr{Obj: kop.RefTriggerPlayer})
	if !s.sweepReplCompete(k, NoSlot) {
		t.Fatal("two marks do not compete in the sweep guard")
	}
	s, k = markedPathState(true)
	if !s.sweepReplCompete(k, NoSlot) {
		t.Fatal("a mark and a doubler do not compete in the sweep guard")
	}
}

// TestDmgMarkDirectPaths: replDamage (the direct damage sites) doubles once
// for a lone mark and stalls StallDmgMarkOrder for a mark beside a doubler or
// a second mark (gorge asks the order).
func TestDmgMarkDirectPaths(t *testing.T) {
	s, k := markedPathState(false)
	if n := s.replDamage(k, 0, NoSlot, 1, PlayerTarget0+1, false, 3); n != 6 || s.g.stalled != 0 {
		t.Fatalf("a lone mark: damage %d stalled %d, want 6 / 0", n, s.g.stalled)
	}
	s.res.e.ctrl = 1
	s.dmgMarkAdd(nil, &kop.Instr{Obj: kop.RefTriggerPlayer})
	if n := s.replDamage(k, 0, NoSlot, 1, PlayerTarget0+1, false, 3); n != 0 || s.g.stalled != StallDmgMarkOrder {
		t.Fatalf("two marks: damage %d stalled %d, want 0 / %d", n, s.g.stalled, StallDmgMarkOrder)
	}
	s, k = markedPathState(true)
	if n := s.replDamage(k, 0, NoSlot, 1, PlayerTarget0+1, false, 3); n != 0 || s.g.stalled != StallDmgMarkOrder {
		t.Fatalf("a mark and a doubler: damage %d stalled %d, want 0 / %d", n, s.g.stalled, StallDmgMarkOrder)
	}
}
