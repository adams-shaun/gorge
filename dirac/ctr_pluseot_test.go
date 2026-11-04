package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCounterPlusEOT covers the turn flag's two seats, its stacking (each
// activation is its own replacement effect, so two taps mean plus two) and
// its cleanup expiry.
func TestCounterPlusEOT(t *testing.T) {
	set := cardsettest.Set(t, "boimler")
	deck := []cardset.Entry{
		{Name: "Brad Boimler, Eager Ensign", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 1},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Forest"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	sc := &scen{s: s, k: s.k()}
	mine := sc.slot(sc.perm(id["Grizzly Bears"], 0))
	theirs := sc.slot(sc.perm(id["Grizzly Bears"], 1))

	if got := s.replCounters(sc.k, mine, int(kop.CtrP1P1), 1); got != 1 {
		t.Fatalf("before the flag: %d counters, want 1", got)
	}
	s.res.e.ctrl = 0
	s.ctrPlusEOT(sc.k)
	if got := s.replCounters(sc.k, mine, int(kop.CtrP1P1), 1); got != 2 {
		t.Fatalf("after one activation: %d counters, want 2", got)
	}
	if got := s.replCounters(sc.k, theirs, int(kop.CtrP1P1), 1); got != 1 {
		t.Fatalf("opponent's permanent: %d counters, want 1", got)
	}
	s.ctrPlusEOT(sc.k)
	if got := s.replCounters(sc.k, mine, int(kop.CtrP1P1), 1); got != 3 {
		t.Fatalf("after two activations: %d counters, want 3", got)
	}
	s.cleanupGen(sc.k)
	if s.g.plusCtr[0] != 0 || s.g.plusCtr[1] != 0 {
		t.Fatalf("cleanup left plusCtr %v, want zero", s.g.plusCtr)
	}
	if got := s.replCounters(sc.k, mine, int(kop.CtrP1P1), 1); got != 1 {
		t.Fatalf("after cleanup: %d counters, want 1", got)
	}
}
