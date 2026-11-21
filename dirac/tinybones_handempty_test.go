package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTinybonesHandEmptyPlayerPred pins the kernel arm of Tinybones, Trinket
// Thief's "each opponent with no cards in hand" (gorge
// Player.Opponent+HasCardsInHand_Card_EQ0): OpRepeatEachPlayer over
// RefEachOpponent with PlayerPredHandEmpty binds an opponent whose hand is
// empty, skips the body for one holding a card.
func TestTinybonesHandEmptyPlayerPred(t *testing.T) {
	sc, c := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	s := sc.s
	s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpRepeatEachPlayer, Obj: kop.RefEachOpponent, A: kop.Const(3), C: 1, B: kop.Const(kop.PlayerPredHandEmpty)}

	// Opponent hand empty: the loop binds seat 1 and runs the body.
	if s.pl[1].handN != 0 {
		t.Fatalf("precondition: seat 1 hand = %d, want 0", s.pl[1].handN)
	}
	pc := s.res.pc
	s.exec(sc.k, &in)
	if s.g.eachPl != 1 || s.g.repNext != 255 || s.res.pc == pc+3 {
		t.Fatalf("empty opponent hand: eachPl %d next %d pc %d (start %d), want seat 1 bound and the body run", s.g.eachPl, s.g.repNext, s.res.pc, pc)
	}

	// A card in the opponent's hand: nobody is bound, the body is skipped.
	sc.hand(c["Plains"], 1)
	if s.pl[1].handN != 1 {
		t.Fatalf("precondition: seat 1 hand = %d, want 1", s.pl[1].handN)
	}
	s.g.eachPl, s.g.repNext, s.g.repEnd = 0, 0, 0
	pc = s.res.pc
	s.exec(sc.k, &in)
	if s.g.repEnd != 0 || s.g.eachPl != 0 || s.g.repNext != 0 || s.res.pc != pc+3 {
		t.Fatalf("held card: loop state %d/%d/%d pc %d (start %d), want none bound and the body skipped",
			s.g.eachPl, s.g.repNext, s.g.repEnd, s.res.pc, pc)
	}
}
