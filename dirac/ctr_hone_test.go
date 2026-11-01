package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHonePrepOffOnFrozenPools: FDN and BLB place no hone counter, so
// anyHone stays false and ptGen's CR 122.1j arm never runs there.
func TestHonePrepOffOnFrozenPools(t *testing.T) {
	blbSet := cardsettest.Set(t, "blb")
	fdnSet := cardsettest.Set(t, "fdn")
	for _, f := range []func() (*kop.Table, error){fdnSet.Pool, blbSet.Pool} {
		tb, err := f()
		if err != nil {
			t.Fatal(err)
		}
		g := genPrep{}
		prepHone(tb, &g)
		if g.anyHone {
			t.Fatal("anyHone on a frozen pool")
		}
	}
}

// honeScen is a ctrnamed matchup whose decks carry Dwalin (so prepHone
// enables the read), over the pool's own Mountain library.
func honeScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Dwalin, Weaponmaster", Count: 2}, {Name: "Chainsaw", Count: 2},
		{Name: "Bonesplitter", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 8}}
	kctrnamedSet := cardsettest.Set(t, "ctrnamed")
	tbl, da, _, err := kctrnamedSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true}), id
}

// TestCtrhoneHone: CR 122.1j. Each hone counter on a permanent attached to
// a creature is +1/+0 for it: a counter's effect, so the permanent need not
// be an Equipment or keep its abilities; a non-creature gets nothing.
func TestCtrhoneHone(t *testing.T) {
	sc, id := honeScen(t)
	if !sc.k.gen.anyHone {
		t.Fatal("anyHone unset with Dwalin in the table")
	}
	s := sc.s
	giant := sc.perm(id["Hill Giant"], 0)
	saw := sc.perm(id["Chainsaw"], 0)
	bone := sc.perm(id["Bonesplitter"], 0)
	forest := sc.perm(id["Forest"], 0)
	g, cs, bs, fo := sc.slot(giant), sc.slot(saw), sc.slot(bone), sc.slot(forest)
	chk := func(step string, sl Slot, wp, wt int) {
		t.Helper()
		if p, tt := s.pt(sc.k, sl); p != wp || tt != wt {
			t.Errorf("%s: %d/%d, want %d/%d", step, p, tt, wp, wt)
		}
	}
	chk("Hill Giant alone", g, 3, 3)
	s.perms[cs].attached = g
	s.perms[cs].addCtrN(s, int(kop.CtrRev), 2)
	chk("Chainsaw, REV=2", g, 5, 3)
	s.perms[cs].addCtrN(s, int(kop.CtrHone), 1)
	chk("HONE=1", g, 6, 3)
	s.perms[cs].addCtrN(s, int(kop.CtrHone), 2)
	chk("HONE=3", g, 8, 3)
	s.perms[cs].subCtrN(s, int(kop.CtrHone), 3)
	chk("HONE=0", g, 5, 3)
	s.perms[cs].addCtrN(s, int(kop.CtrHone), 1)
	s.perms[cs].attached = NoSlot
	chk("Chainsaw detached, HONE remains", g, 3, 3)
	s.perms[bs].attached = g
	s.perms[bs].addCtrN(s, int(kop.CtrHone), 1)
	chk("Bonesplitter + HONE=1", g, 6, 3)
	s.perms[bs].ovr.flags |= ovLoseAll
	chk("Bonesplitter's abilities lost", g, 4, 3)
	s.perms[bs].ovr.flags &^= ovLoseAll
	s.perms[bs].attached = fo
	chk("hone on a Forest-attached Equipment", fo, 0, 0)
	chk("Hill Giant after the move", g, 3, 3)
}

// TestCtrhoneDwalin: Dwalin's enters trigger puts a hone counter on each
// Equipment (kgen's OpPutCounterAll); the attached Bonesplitter is then
// +2/+0 and +1/+0, and the power tracks Giant Growth and a count change.
func TestCtrhoneDwalin(t *testing.T) {
	sc, id := honeScen(t)
	s := sc.s
	bone := sc.perm(id["Bonesplitter"], 0)
	giant := sc.perm(id["Hill Giant"], 0)
	sc.lands(0, id["Mountain"], 3)
	sc.lands(0, id["Plains"], 3)
	sc.lands(0, id["Forest"], 1)
	dw := sc.hand(id["Dwalin, Weaponmaster"], 0)
	sc.castCard(t, dw)
	sc.resolveAll(t)
	if !sc.onBF(dw) {
		t.Fatal("Dwalin did not resolve")
	}
	if n := s.Counters(bone, kop.CtrHone); n != 1 {
		t.Fatalf("Bonesplitter has %d HONE, want 1", n)
	}
	g, bs := sc.slot(giant), sc.slot(bone)
	s.perms[bs].attached = g
	if p, _ := s.pt(sc.k, g); p != 6 {
		t.Fatalf("Hill Giant power %d, want 3+2+1 = 6", p)
	}
	gg := sc.hand(id["Giant Growth"], 0)
	sc.castAt(t, gg, giant)
	if p, _ := s.pt(sc.k, g); p != 9 {
		t.Fatalf("Hill Giant power %d after Giant Growth, want 9", p)
	}
	s.perms[bs].subCtrN(s, int(kop.CtrHone), 1)
	if p, _ := s.pt(sc.k, g); p != 8 {
		t.Fatalf("Hill Giant power %d after the hone counter goes, want 8", p)
	}
}
