package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// counterKindsScen is a main-phase scenario over the counterkinds fixture
// pool, libraries of Plains.
func counterKindsScen(t *testing.T) (*scen, func(string) CardID) {
	t.Helper()
	set := cardsettest.Set(t, "counterkinds")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	return newScenTableLib(t, tbl, id("Plains"), Config{NoElide: true}), id
}

// TestDifferentCounterKinds: ExCounterKinds counts each counter kind with a
// positive count once over the creatures the controller controls (the +1/+1
// field, the general slots and the overflow table), never a non-creature's
// or an opponent's counters, nor gorge's marker flags; Hundred-Battle
// Veteran's self-pump switches on at three kinds.
func TestDifferentCounterKinds(t *testing.T) {
	sc, id := counterKindsScen(t)
	s, k := sc.s, sc.k
	k.t.Filters = append(k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(k.t.Filters) - 1)
	count := func() int {
		x := kop.Expr{Kind: kop.ExCounterKinds, F: f}
		return s.exprCold(k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	vet := sc.slot(sc.perm(id("Hundred-Battle Veteran"), 0))
	ag := sc.slot(sc.perm(id("Grizzly Bears"), 0))
	land := sc.slot(sc.perm(id("Plains"), 0))
	opp := sc.slot(sc.perm(id("Grizzly Bears"), 1))
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
