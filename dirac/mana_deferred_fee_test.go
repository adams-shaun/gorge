package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckDeferredFeeSpend is the kernel-side half of TestDeferredSourceForgottenOnFeeSpend
// (dirac_test). Gilanra's {G} floats (the window's deferred gap) and a fee then
// spends it from the pool: Abzan Devotee's {1} filter fee (filtDo), then
// Tarnation Vista's {1} each-colour fee (eachProduce). gorge's payment consumes
// Gilanra's provenance batch there, so a later cast paid with other {G} is not
// Gilanra's and draws nothing. A creature source sets no poolArt bit, so only
// the fee's own spend can forget it (deferSync).
func CheckDeferredFeeSpend(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	floated := func(sc *scen, what string) Obj {
		g := sc.perm(id["Gilanra, Caller of Wirewood"], 0)
		sc.s.floatDeferred(sc.k, 0, sc.slot(g))
		if sc.s.g.defSrc[0] != uint8(sc.slot(g))+1 || sc.s.pl[0].pool[kop.G] != 1 || sc.s.g.poolArt[0] != 0 {
			t.Fatalf("%s precondition: defSrc %d, pool G %d, poolArt %d", what, sc.s.g.defSrc[0], sc.s.pl[0].pool[kop.G], sc.s.g.poolArt[0])
		}
		return g
	}
	// wurmPaid casts Craw Wurm ({4}{G}{G}) from {G} floated by other sources:
	// its build must name no deferred source
	wurmPaid := func(sc *scen, what string) {
		sc.s.pl[0].pool[kop.G] += 6 - sc.s.pl[0].pool[kop.G]
		wurm := id["Craw Wurm"]
		o := sc.obj(wurm, 0)
		sc.kcastBuild(o, wurm, 0)
		cost := sc.k.t.Cards[wurm].Cost
		sc.s.payRider(sc.k, 0, &cost, 0, o, false)
		if got := sc.s.bld().obsSl; got != 0 {
			t.Errorf("%s: obsSl %d, the Wurm's {G} is not Gilanra's", what, got)
		}
	}

	// (a) a filter's fee
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	dev := sc.perm(id["Abzan Devotee"], 0)
	floated(sc, "filter")
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(dev)) {
		t.Fatal("Abzan Devotee is no filter gap")
	}
	sc.s.filtDo(sc.k, 0, filtStep{sl: sc.slot(dev), gap: true, ans: 1 + kop.G})
	if sc.s.pl[0].pool[kop.G] != 1 {
		t.Fatalf("filter: pool G %d after the Devotee's activation, want its one {G} (the fee spent Gilanra's)", sc.s.pl[0].pool[kop.G])
	}
	if sc.s.g.defSrc[0] != 0 {
		t.Errorf("filter: defSrc %d after the Devotee's fee spent Gilanra's {G}: want forgotten", sc.s.g.defSrc[0])
	}
	wurmPaid(sc, "filter")

	// (b) an each-colour ability's fee
	sc = newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	vista := sc.perm(id["Tarnation Vista"], 0)
	vsl := sc.slot(vista)
	d := sc.s.Step() // its as-enters colour choice
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && a.Mode() == 3 {
			sc.s.Apply(a)
			break
		}
	}
	sc.s.perms[vsl].flags &^= pfTapped // it enters tapped
	if sc.s.eachFee(sc.k, vsl) != 1 {
		t.Fatalf("each: the Vista's fee %d, want 1", sc.s.eachFee(sc.k, vsl))
	}
	floated(sc, "each")
	sc.s.perms[vsl].flags |= pfTapped
	sc.s.eachProduce(sc.k, 0, vsl, 1)
	if sc.s.pl[0].pool[kop.G] != 1 {
		t.Fatalf("each: pool G %d after the Vista's each-colour tap, want its one {G} (the fee spent Gilanra's)", sc.s.pl[0].pool[kop.G])
	}
	if sc.s.g.defSrc[0] != 0 {
		t.Errorf("each: defSrc %d after the Vista's fee spent Gilanra's {G}: want forgotten", sc.s.g.defSrc[0])
	}
	wurmPaid(sc, "each")
}
