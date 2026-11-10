package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

func turnReplScen(t *testing.T, pool string, names ...string) (*State, *scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Synth(t, pool)
	var deck []cardset.Entry
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
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
		lib[i] = id[names[0]]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	return s, &scen{s: s, k: s.k()}, id
}

// TestTurnReplPlusCreatureCtr: Prairie Dog's one more +1/+1 counter on a
// creature its controller controls: only +1/+1, only the controller's side,
// stacking per activation, gone at the cleanup.
func TestTurnReplPlusCreatureCtr(t *testing.T) {
	s, sc, id := turnReplScen(t, "std-otj", "Prairie Dog")
	dog := id["Prairie Dog"]
	mine, theirs := sc.slot(sc.perm(dog, 0)), sc.slot(sc.perm(dog, 1))
	put := func(sl Slot, kind kop.CounterKind) int { return s.replCounters(sc.k, sl, int(kind), 1) }
	if got := put(mine, kop.CtrP1P1); got != 1 {
		t.Fatalf("before the effect: %d counters, want 1", got)
	}
	s.res.e.ctrl, s.res.e.obj = 0, sc.s.perms[mine].obj
	in := &kop.Instr{C: kop.TurnReplPlusCreatureCtr}
	s.turnRepl(sc.k, in)
	if got := put(mine, kop.CtrP1P1); got != 2 {
		t.Fatalf("after one activation: %d +1/+1 counters, want 2", got)
	}
	if got := put(mine, kop.CtrM1M1); got != 1 {
		t.Fatalf("a -1/-1 counter: %d, want 1 (Prairie Dog names +1/+1 only)", got)
	}
	if got := put(theirs, kop.CtrP1P1); got != 1 {
		t.Fatalf("the opponent's creature: %d counters, want 1", got)
	}
	s.turnRepl(sc.k, in)
	if got := put(mine, kop.CtrP1P1); got != 3 {
		t.Fatalf("after two activations: %d counters, want 3", got)
	}
	s.cleanupGen(sc.k)
	if got := put(mine, kop.CtrP1P1); got != 1 {
		t.Fatalf("after cleanup: %d counters, want 1", got)
	}
}

// TestTurnReplExileOpp: Malicious Eclipse's scope exiles an opponent's
// creature that dies this turn, one that entered after the effect included,
// and not the controller's own; the cleanup ends it.
func TestTurnReplExileOpp(t *testing.T) {
	// a vanilla creature of the pool beside the Eclipse
	vanilla := ""
	set := cardsettest.Synth(t, "std-lci")
	for i, n := range set.Names()[:set.NumCards()] {
		if d := set.Def(i); d.Types&kop.TCreature != 0 && len(d.Abilities) == 0 && d.Keywords == 0 && !set.Inert(i) {
			if _, _, _, err := set.Matchup([]cardset.Entry{{Name: n, Count: 1}}, []cardset.Entry{{Name: n, Count: 1}}); err == nil {
				vanilla = n
				break
			}
		}
	}
	if vanilla == "" {
		t.Fatal("no vanilla creature in std-lci")
	}
	s, sc, id := turnReplScen(t, "std-lci", "Malicious Eclipse", vanilla)
	k := sc.k
	if k.gen.ix[mechExileEx] == arenaNone {
		t.Fatal("the table's Eclipse does not reserve the exile-instead table")
	}
	mine := sc.slot(sc.perm(id[vanilla], 0))
	s.res.e.ctrl = 0
	if z := s.dieZone(k, mine); z != kop.ZoneGraveyard {
		t.Fatalf("before the effect: %v, want the graveyard", z)
	}
	s.turnRepl(k, &kop.Instr{C: kop.TurnReplExileOpp})
	late := sc.slot(sc.perm(id[vanilla], 1)) // enters after the effect resolved
	if z := s.dieZone(k, late); z != kop.ZoneExile {
		t.Fatalf("an opposing creature that entered after: %v, want exile", z)
	}
	if z := s.dieZone(k, mine); z != kop.ZoneGraveyard {
		t.Fatalf("the controller's own creature: %v, want the graveyard", z)
	}
	s.cleanupGen(k)
	if z := s.dieZone(k, late); z != kop.ZoneGraveyard {
		t.Fatalf("after cleanup: %v, want the graveyard", z)
	}
}
