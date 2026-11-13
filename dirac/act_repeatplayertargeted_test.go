package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRepeatEachTargetedOrder: gorge's RepeatEach RepeatPlayers$ Targeted
// (repeatPlayers' add(c.Targets)) repeats over a target slot's player
// targets in AliveFrom(controller) order -- NOT pick order -- a player who
// has left dropped, and no targets skip the body.
func TestRepeatEachTargetedOrder(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	s := sc.s
	in := kop.Instr{Op: kop.OpRepeatEachPlayer, Obj: kop.RefTarget0, A: kop.Const(0), C: 1}

	// one target, the opponent: the loop runs once over it
	s.res.e.tgt[0] = uint8(PlayerTarget0 + 1)
	s.exec(sc.k, &in)
	if s.g.eachPl != 1 || s.g.repNext != 255 {
		t.Fatalf("one target: eachPl %d next %d, want 1 then none", s.g.eachPl, s.g.repNext)
	}

	// two targets picked opponent-first: the body runs controller-first
	xt := s.xtMut(&s.res.e)
	xt.n = 1
	xt.slot[0] = 0
	xt.code[0] = uint8(PlayerTarget0) // the controller, picked second
	s.g.repEnd = 0
	s.exec(sc.k, &in)
	if s.g.eachPl != 0 || s.g.repNext != 1 {
		t.Fatalf("two targets picked 1,0: eachPl %d next %d, want the controller (0) first", s.g.eachPl, s.g.repNext)
	}

	// no player target left: the body is skipped through the
	// no-first-player path
	xt.n = 0
	s.res.e.tgt[0] = uint8(NoTarget)
	s.g.eachPl, s.g.repNext, s.g.repEnd = 0, 0, 0
	in.A = kop.Const(3)
	pc := s.res.pc
	s.exec(sc.k, &in)
	if s.g.repEnd != 0 || s.g.eachPl != 0 || s.g.repNext != 0 {
		t.Fatalf("no target: loop state %d/%d/%d, want none bound", s.g.eachPl, s.g.repNext, s.g.repEnd)
	}
	if s.res.pc != pc+3 {
		t.Fatalf("no target: pc %d, want %d (the body skipped)", s.res.pc, pc+3)
	}
}
