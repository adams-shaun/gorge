package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPutCounterAllRememberPutOnlyLanded: the overload each-variant of
// PutCounter (PutCounterAll B=RememberPut) remembers exactly the recipients
// whose counter actually landed, after replacements -- gorge's rememberPlaced
// over its placed list. A CantPutCounter prohibition (Blossombind) swallows
// the host's counter, so the host must not join the remembered set that a
// following Defined$ Remembered instruction (Fangs of Kalonia's
// MultiplyCounter, Spectacular Showdown's Goad) reads.
func TestPutCounterAllRememberPutOnlyLanded(t *testing.T) {
	set := cardsettest.Set(t, "anykindcounters")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Skipf("%s not in the anykindcounters pool", name)
		}
		return CardID(set.PoolID(i))
	}
	sc := newScenTableLib(t, tbl, lookup("Plains"), Config{NoElide: true})
	if !sc.k.gen.anyCtrBlock {
		t.Fatal("prep missed the pool's StatCantPutCounter")
	}
	host := sc.perm(lookup("Grizzly Bears"), 0)
	other := sc.perm(lookup("Grizzly Bears"), 0)
	aura := sc.perm(lookup("Blossombind"), 0)
	sc.s.perms[sc.slot(aura)].attached = sc.slot(host)
	if !sc.s.putCounterBlocked(sc.k, sc.slot(host), int(kop.CtrP1P1)) {
		t.Fatal("putCounterBlocked missed the enchanted creature")
	}

	// A creature-only filter over the pool's own table: both Grizzly Bears
	// match, the Aura does not.
	f := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	in := kop.Instr{Op: kop.OpPutCounterAll, A: kop.Const(1), B: kop.Const(kop.PutCtrRememberPut),
		C: int16(kop.CtrP1P1), F: f}

	sc.s.putCounterAll(sc.k, &in)
	if n := sc.s.perms[sc.slot(other)].ctrLow(int(kop.CtrP1P1)); n != 1 {
		t.Fatalf("unblocked creature holds %d +1/+1 counters, want 1", n)
	}
	if n := sc.s.g.nrem; n != 1 {
		t.Fatalf("RememberPut remembered %d recipients, want only the 1 that landed", n)
	}
	if sc.s.g.rem[0] != other {
		t.Fatalf("RememberPut remembered object %d, want the unblocked creature %d", sc.s.g.rem[0], other)
	}

	// The mutation switch covers the sweep arm too.
	sc.s.g.nrem = 0
	defer func() { DebugRemPutNever = false }()
	DebugRemPutNever = true
	sc.s.putCounterAll(sc.k, &in)
	if n := sc.s.g.nrem; n != 0 {
		t.Fatalf("DebugRemPutNever remembered %d recipients", n)
	}
}
