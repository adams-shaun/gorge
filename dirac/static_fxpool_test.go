package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFxPoolHashStable pins the position hashes (ObjHash, featCtxPerm) and
// the layer-6 loss reads of a board carrying override records (until end of
// turn and permanent, a merged repeat) and one-shot "loses all abilities"
// effects, before and after cleanup. These live in
// the effect pool (fxRec); the digest was taken on the code before it, so
// the hash inputs stay equivalent (search transpositions and feature
// caches see the same keys). Re-pinned for copy exceptions: the FDN table's
// copy-exception overrides (Abyssal Harvester, Chandra, Electroduplicate)
// now sit in Table.Overrides ahead of Witness Protection's, so its
// override records' idx (hashed) moved by 3; nothing else changed.
func TestFxPoolHashStable(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Celestial Armor", Count: 1},
		{Name: "Ambush Wolf", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
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
	wp := id["Witness Protection"]
	ov := -1
	for _, a := range tbl.Cards[wp].Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatOverride && a.Static.Override > 0 {
			ov = int(a.Static.Override - 1)
		}
	}
	if ov < 0 {
		t.Skip("Witness Protection has no override static")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	w1 := sc.perm(id["Ambush Wolf"], 0)
	w2 := sc.perm(id["Ambush Wolf"], 1)
	w3 := sc.perm(id["Ambush Wolf"], 0)
	armor := sc.perm(id["Celestial Armor"], 0)
	sc.s.perms[sc.slot(armor)].attached = sc.slot(w3)
	sc.s.applyOverride(sc.k, sc.slot(w1), ov, wp, true)
	sc.s.applyOverride(sc.k, sc.slot(w1), ov, wp, true)
	sc.s.applyOverride(sc.k, sc.slot(w2), ov, wp, false)
	lose := func(o Obj, eot bool) {
		sc.s.res.e.obj, sc.s.res.e.srcVer, sc.s.res.e.ctrl = o, sc.s.objs[o].ver, 0
		in := kop.Instr{Op: kop.OpLoseAbilities, Obj: kop.RefSelf}
		if eot {
			in.C = 1
		}
		sc.s.exec(sc.k, &in)
	}
	lose(w3, true)
	lose(w2, false)
	if _, lost := sc.s.lossAt(sc.k, sc.slot(w2)); !lost {
		t.Fatal("the one-shot loss did not apply")
	}
	var h uint64
	digest := func() {
		for o := Obj(0); o < Obj(sc.s.nobj); o++ {
			h = hmix(h, sc.s.ObjHash(o))
		}
		for m := sc.s.occ; m.any(); m = m.rest() {
			sl := m.first()
			h = hmix(h, sc.s.featCtxPerm(sl))
			ts, lost := sc.s.lossAt(sc.k, sl)
			kw, _ := sc.s.FeatKeywords(sc.s.perms[sl].obj)
			b := uint64(0)
			if lost {
				b = 1
			}
			h = hmix(h, uint64(ts)|b<<16|uint64(kw)<<24)
		}
	}
	digest()
	sc.s.finishCleanup(sc.k)
	digest()
	lose(w1, false)
	digest()
	want := uint64(fxPoolHashPin)
	if MaxSeats > 2 {
		want = fxPoolHashPinMP
	}
	if h != want {
		t.Fatalf("hash digest %#x, want %#x", h, want)
	}
}

// The pin moved when ovRec grew P/T and a repeated P/T setter became the
// newest record: the hash covers the override records,
// so the extra fields and their order change the digest.
const fxPoolHashPin = 0x653f405b2cffb158

// fxPoolHashPinMP is the dirac_mp build's pin: ObjHash reads the permanent
// whole, and its code fields are MaxSeats-dependent widths (TestPermSize), as
// TestOverrideRecordsFitEight's ObjHash pin.
const fxPoolHashPinMP = 0x5924b07694b5a87a
