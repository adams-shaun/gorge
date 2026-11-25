package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// lossCopyEnter puts the first card named name from seat p's library onto
// the battlefield under p.
func lossCopyEnter(t *testing.T, s *State, k *prep, p uint8, name string) Obj {
	t.Helper()
	pl := &s.pl[p]
	for i := pl.libLo; i < pl.libHi; i++ {
		o := pl.lib[i]
		if k.t.Cards[s.objs[o].card].Name == name {
			s.detach(o)
			s.enter(k, o, p, false, kop.ZoneLibrary)
			return o
		}
	}
	t.Fatalf("seat %d: no %s in the library", p, name)
	return NoObj
}

// lossCopyTokens is the token copies seat p controls, by card.
func lossCopyTokens(s *State, p uint8) map[CardID]int {
	got := map[CardID]int{}
	for m := s.occ; m.any(); m = m.rest() {
		pm := &s.perms[m.first()]
		if pm.ctrl == p && s.objs[pm.obj].of&ofCopy != 0 {
			got[s.objs[pm.obj].card]++
		}
	}
	return got
}

// TestContractLossCopiesLoserBoard: seat 1 holds a contract counter that
// seat 0's Miss Highwater Effect put there and loses the game while the pod
// goes on; seat 0 gets one token copy of each artifact and creature seat 1
// controlled, none of its land, and the record is spent.
func TestContractLossCopiesLoserBoard(t *testing.T) {
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
	s := new(State)
	s.InitN(tbl, cds, MaxSeats, 1, Config{NoElide: true})
	k := s.k()
	if s.contractCold() == nil {
		t.Fatal("the table keeps no contract record")
	}
	bear1 := lossCopyEnter(t, s, k, 1, "Birds of Paradise")
	rock := lossCopyEnter(t, s, k, 1, "Sol Ring")
	bear2 := lossCopyEnter(t, s, k, 1, "Birds of Paradise")
	land := lossCopyEnter(t, s, k, 1, "Island")
	// seat 2's creature is not seat 1's to copy
	lossCopyEnter(t, s, k, 2, "Birds of Paradise")

	// the preconditions the assertions depend on
	nonLand, other := 0, 0
	for _, o := range []Obj{bear1, rock, bear2, land} {
		if s.typesAt(k, s.objs[o].slot)&(kop.TCreature|kop.TArtifact) != 0 {
			nonLand++
		} else {
			other++
		}
	}
	if nonLand != 3 || other != 1 {
		t.Fatalf("seat 1 holds %d artifact-or-creature permanents and %d others, want 3 and 1", nonLand, other)
	}
	s.addContract(k, 1, 1)
	s.setContractOwed(1, 0, bear1)
	if c, _, ok := s.contractOwedBy(1); !ok || c != 0 || s.contracts(1) == 0 {
		t.Fatalf("owed record before the loss: seat %d ok %v, contracts %d", c, ok, s.contracts(1))
	}
	if n := len(lossCopyTokens(s, 0)); n != 0 {
		t.Fatalf("seat 0 holds %d token kinds before the loss, want none", n)
	}

	pre := s.outMask()
	s.pl[1].lost = true
	s.sweepDeparted(k, pre)

	// Gorge puts the LosesGame trigger on the stack; the kernel stages the
	// copy and enqueues the same delayed trigger. The sweep alone makes no
	// token.
	if n := len(lossCopyTokens(s, 0)); n != 0 {
		t.Fatalf("the loss sweep minted %d token kinds before the trigger resolved", n)
	}
	if s.npend != 1 || s.pend[0].trigAmt != contractLossTrigAmt || s.pend[0].ctrl != 0 ||
		s.pend[0].trigPl != 1 || s.pend[0].obj != bear1 {
		t.Fatalf("pending loss-copy trigger = %+v (n=%d), want one contractLossTrigAmt for seat 1 sourced at %d",
			s.pend[:s.npend], s.npend, bear1)
	}
	pt := s.pend[0]
	s.npend = 0
	s.resolveDelayed(k, &stackEntry{kind: seTrigger, obj: pt.obj, srcVer: pt.srcVer, card: pt.card,
		ab: pt.ab, ctrl: pt.ctrl, trigPl: pt.trigPl, trigAmt: pt.trigAmt})

	got := lossCopyTokens(s, 0)
	bears, mana := s.objs[bear1].card, s.objs[rock].card
	if got[bears] != 2 || got[mana] != 1 || len(got) != 2 {
		t.Fatalf("seat 0's token copies = %v, want 2 of card %d (Birds of Paradise) and 1 of card %d (Sol Ring)", got, bears, mana)
	}
	if _, _, ok := s.contractOwedBy(1); ok {
		t.Fatal("the owed record outlived the loss")
	}
	if len(lossCopyTokens(s, 2)) != 0 || len(lossCopyTokens(s, 3)) != 0 {
		t.Fatal("a token went to a seat other than the Effect's controller")
	}
	// a second sweep owes nothing more: the staged record was spent
	s.sweepDeparted(k, s.outMask())
	if s.npend != 0 {
		t.Fatalf("a second sweep queued %d more triggers", s.npend)
	}
	if again := lossCopyTokens(s, 0); again[bears] != 2 || again[mana] != 1 {
		t.Fatalf("a second sweep changed the copies: %v", again)
	}
}

// TestContractPutRecordsTheLossCopyOnlyWhenArmed: the ext put makes the owed
// record only with the op's C set (kgen's contractLossEffect arms Miss
// Highwater's put); with C zero it gives the counter and records nothing.
func TestContractPutRecordsTheLossCopyOnlyWhenArmed(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	entries := []cardset.Entry{{Name: "Miss Highwater", Count: 4}, {Name: "Island", Count: 20}}
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
	s := new(State)
	s.InitN(tbl, cds, MaxSeats, 1, Config{NoElide: true})
	k := s.k()
	put := func(c int16) {
		in := kop.ExtInstr(kop.ExtPutContract)
		in.Obj, in.B, in.C = kop.RefEachOpponent, kop.Const(1), c
		s.res.e.ctrl = 0
		s.putContract(k, &in)
	}
	put(0)
	if s.contracts(1) != 1 {
		t.Fatalf("contracts(1) = %d after the put, want 1", s.contracts(1))
	}
	if _, _, ok := s.contractOwedBy(1); ok {
		t.Fatal("an unarmed put recorded the loss copy")
	}
	put(1)
	if c, _, ok := s.contractOwedBy(1); !ok || c != 0 {
		t.Fatalf("an armed put's record: seat %d ok %v, want seat 0", c, ok)
	}
}
