package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newStdTDMScen is a main-phase scenario over the std-tdm synthetic pool's
// matchup of deck vs deck, libraries of Plains; a deck card the pool does
// not support fails the test rather than skipping it.
func newStdTDMScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Synth(t, "std-tdm")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in std-tdm", e.Name)
		}
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
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// TestDifferentCounterKinds: ExCounterKinds counts each counter kind with a
// positive count once over the creatures the controller controls (the +1/+1
// field, the general slots and the overflow table), never a non-creature's
// or an opponent's counters, nor gorge's marker flags; Hundred-Battle
// Veteran's self-pump switches on at three kinds.
func TestDifferentCounterKinds(t *testing.T) {
	sc, id := newStdTDMScen(t, []cardset.Entry{{Name: "Hundred-Battle Veteran", Count: 1}, {Name: "Agent of Kotis", Count: 2}, {Name: "Plains", Count: 8}})
	s, k := sc.s, sc.k
	k.t.Filters = append(k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(k.t.Filters) - 1)
	count := func() int {
		x := kop.Expr{Kind: kop.ExCounterKinds, F: f}
		return s.exprCold(k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	vet := sc.slot(sc.perm(id["Hundred-Battle Veteran"], 0))
	ag := sc.slot(sc.perm(id["Agent of Kotis"], 0))
	land := sc.slot(sc.perm(id["Plains"], 0))
	opp := sc.slot(sc.perm(id["Agent of Kotis"], 1))
	put := func(sl Slot, c kop.CounterKind, n int) {
		t.Helper()
		if !s.perms[sl].setCtrN(s, int(c), n) {
			t.Fatalf("setCtrN(%d, %d) failed", c, n)
		}
	}
	vetPT := func() (int, int) { return s.ptGen(k, vet) }
	if got := count(); got != 0 {
		t.Fatalf("no counters: %d kinds, want 0", got)
	}
	if p, tg := vetPT(); p != 4 || tg != 2 {
		t.Fatalf("no counters: Veteran %d/%d, want the printed 4/2", p, tg)
	}
	put(vet, kop.CtrStun, 1)
	put(ag, kop.CtrP1P1, 2)
	if got := count(); got != 2 {
		t.Fatalf("stun + +1/+1 over two creatures: %d kinds, want 2", got)
	}
	put(ag, kop.CtrStun, 1) // the same kind on a second creature counts once
	put(land, kop.CtrCharge, 1)
	put(opp, kop.CtrFlying, 1)
	if got := count(); got != 2 {
		t.Fatalf("a repeated kind, a land's and an opponent's counters: %d kinds, want 2", got)
	}
	s.perms[vet].flags |= pfRegen | pfDeathtouched // gorge's markers are no counter kinds
	if got := count(); got != 2 {
		t.Fatalf("marker flags moved the count to %d", got)
	}
	if p, tg := vetPT(); p != 4 || tg != 2 {
		t.Fatalf("two kinds: Veteran %d/%d, want 4/2 (pump off)", p, tg)
	}
	put(ag, kop.CtrShield, 1)
	if got := count(); got != 3 {
		t.Fatalf("stun, +1/+1, shield: %d kinds, want 3", got)
	}
	if p, tg := vetPT(); p != 6 || tg != 6 {
		t.Fatalf("three kinds: Veteran %d/%d, want 6/6 (pump on)", p, tg)
	}
	// slots full: the further kinds live in the overflow table
	put(ag, kop.CtrOil, 1)
	put(ag, kop.CtrSpore, 1)
	put(ag, kop.CtrQuest, 1)
	if s.g.nctrOver == 0 {
		t.Fatal("precondition: no counter kind reached the overflow table")
	}
	if got := count(); got != 6 {
		t.Fatalf("three overflow kinds: %d kinds, want 6", got)
	}
	put(ag, kop.CtrQuest, 0)
	put(ag, kop.CtrShield, 0) // a removed kind kept at 0 does not count
	if got := count(); got != 4 {
		t.Fatalf("two kinds removed: %d kinds, want 4", got)
	}
}
