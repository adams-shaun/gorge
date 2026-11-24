package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// lossMarksSetup is TestContractLossCopiesLoserBoard's four-seat table with
// seat 0's Miss Highwater on the battlefield, seat 1 contracted and owing its
// loss copy to seat 0, and seat 1 holding two Birds of Paradise and a Sol Ring.
func lossMarksSetup(t *testing.T) (s *State, k *prep, mhw Obj, bears, mana CardID) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	entries := []cardset.Entry{{Name: "Miss Highwater", Count: 4}, {Name: "Birds of Paradise", Count: 10},
		{Name: "Sol Ring", Count: 6}, {Name: "Island", Count: 20}}
	decks := make([][]cardset.Entry, MaxSeats)
	for p := range decks {
		decks[p] = entries
	}
	tbl, da, err := set.MatchupN(decks)
	if err != nil {
		t.Fatal(err)
	}
	var cds [MaxSeats][]CardID
	for p := range cds {
		cds[p] = da[p]
	}
	s = new(State)
	s.InitN(tbl, cds, MaxSeats, 1, Config{NoElide: true})
	k = s.k()
	mhw = lossCopyEnter(t, s, k, 0, "Miss Highwater")
	b1 := lossCopyEnter(t, s, k, 1, "Birds of Paradise")
	rock := lossCopyEnter(t, s, k, 1, "Sol Ring")
	lossCopyEnter(t, s, k, 1, "Birds of Paradise")
	s.addContract(k, 1, 1)
	s.setContractOwed(1, 0, mhw)
	if c, ok := s.contractOwedBy(1); !ok || c != 0 {
		t.Fatalf("owed record before the loss: seat %d ok %v", c, ok)
	}
	return s, k, mhw, s.objs[b1].card, s.objs[rock].card
}

// lossMarksRun resolves what the loss put on the stack.
func lossMarksRun(s *State) {
	for i := 0; i < 40 && (s.npend > 0 || s.sp > 0) && !s.Stalled(); i++ {
		d := s.Step()
		if d.Kind == DkPriority {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
}

// TestContractLossCopiesOnStateBasedLifeLoss: the contracted seat loses to the
// life-total state-based action (CR 704.5a), a loss site that does not go
// through lose(); the loss copy fires there too, as gorge's PlayerLost emit
// queues it for every loss.
func TestContractLossCopiesOnStateBasedLifeLoss(t *testing.T) {
	s, k, _, bears, mana := lossMarksSetup(t)
	pre := s.outMask()
	s.pl[1].life = 0
	s.sba(k)
	if !s.pl[1].lost {
		t.Fatal("the life-total SBA did not mark seat 1 lost")
	}
	s.sweepDeparted(k, pre)
	lossMarksRun(s)
	got := lossCopyTokens(s, 0)
	if got[bears] != 2 || got[mana] != 1 || len(got) != 2 {
		t.Fatalf("seat 0's token copies = %v, want 2 of card %d (Birds of Paradise) and 1 of card %d (Sol Ring)", got, bears, mana)
	}
}

// TestContractLossCopyDroppedForANewSource: the Effect's source leaves and
// returns between the fire and the resolution (a new object, ver moved on):
// gorge's registration ended with the old one, so no copy is made.
func TestContractLossCopyDroppedForANewSource(t *testing.T) {
	s, k, mhw, _, _ := lossMarksSetup(t)
	pre := s.outMask()
	s.lose(1)
	s.sweepDeparted(k, pre)
	// run to the point where the fired copy is on the stack awaiting a pass
	for i := 0; i < 40 && s.sp == 0 && !s.Stalled(); i++ {
		d := s.Step()
		if d.Kind == DkPriority && s.sp == 0 {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
	}
	if s.sp == 0 {
		t.Fatal("the fired copy never reached the stack, so the resolution check is not reached")
	}
	s.objs[mhw].ver++ // left and came back
	lossMarksRun(s)
	if got := lossCopyTokens(s, 0); len(got) != 0 {
		t.Fatalf("a copy was made for a source that left and came back: %v", got)
	}
}
