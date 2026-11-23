package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEnterAsksDefaultAfterControllerLeftCard: a sweep-returned card whose
// enter program carries ExtChooseCardEnter (Dauntless Bodyguard's mandatory
// choice of another creature you control) records the pose's first candidate.
// gorge's no-host ChooseCard fallback takes choices[:min] -- the first
// candidate, the one lowering being mandatory -- where the no-run path's
// queue-reading option builder (enterCardOptions, reading a queue the path
// never filled) offered nothing and the chosen byte was left recording no
// choice.
func TestEnterAsksDefaultAfterControllerLeftCard(t *testing.T) {
	sc, guard := leftReturnScen(t, "bodyguard", "Dauntless Bodyguard", "Plains",
		cardset.Entry{Name: "Savannah Lions", Count: 1})
	s := sc.s

	// seat 2's Savannah Lions is the one other creature the returned
	// Bodyguard can choose (Choices$ Creature.YouCtrl+Other)
	var lions = Obj(NoObj)
	{
		pl := &s.pl[2]
		for i := pl.libLo; i < pl.libHi; i++ {
			if s.k().t.Cards[s.objs[pl.lib[i]].card].Name == "Savannah Lions" {
				lions = pl.lib[i]
				break
			}
		}
		if lions == Obj(NoObj) {
			t.Fatal("seat 2: no Savannah Lions in the library")
		}
		s.detach(lions)
		s.enter(sc.k, lions, 2, false, kop.ZoneLibrary)
	}

	pre := s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(sc.k, pre)
	if s.g.nask != 0 {
		t.Fatalf("the sweep queued %d asks, want none", s.g.nask)
	}
	sl := leftReturnField(t, sc, guard)
	if s.perms[sl].chosen != objWord(lions) {
		t.Fatalf("the returned Bodyguard chose %d, want %d: the first candidate (seat 2's Savannah Lions)", s.perms[sl].chosen, objWord(lions))
	}
	if d := s.Step(); d.Kind == DkChoose {
		t.Fatalf("a decision was posed after the sweep: kind %d sub %d player %d n %d, want the game carrying on unasked", d.Kind, d.Sub, d.Player, d.N)
	}
}

// TestEnterAsksDefaultAfterControllerLeftRLine: a sweep-returned card whose
// R: AfterEnter program opens with its as-enters choice (Banner of Kinship's
// type) is not one of gorge's walker-posed CR 614.12 entry choices --
// enterChoiceKind skips R: lines, so gorge never poses that choice with the
// card in its origin zone. With no resolution run serving the return, the
// line's body runs straight off the entry emit and defaults the choice to the
// first option, unasked: before, it stayed queued and was posed as a decision.
func TestEnterAsksDefaultAfterControllerLeftRLine(t *testing.T) {
	sc, banner := leftReturnScen(t, "fdn", "Banner of Kinship", "Island")
	s := sc.s
	pre := s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(sc.k, pre)
	if s.g.nask != 0 {
		t.Fatalf("the sweep queued %d asks, want none", s.g.nask)
	}
	sl := leftReturnField(t, sc, banner)
	if s.perms[sl].chosen == 0 {
		t.Fatal("the returned Banner of Kinship chose no type: the R: line's choice was not defaulted")
	}
	if d := s.Step(); d.Kind == DkChoose {
		t.Fatalf("a decision was posed after the sweep: kind %d sub %d player %d n %d, want the game carrying on unasked", d.Kind, d.Sub, d.Player, d.N)
	}
}
