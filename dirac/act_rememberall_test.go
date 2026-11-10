package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestPumpAllRemembers: kop.ExtRememberAll (PumpAll RememberPumped$) puts
// every battlefield permanent its filter matches into the remembered set,
// seat 0's first in timestamp order, then seat 1's (gorge effPumpAll's walk);
// a permanent the filter misses stays out.
func TestPumpAllRemembers(t *testing.T) {
	var creature kop.FilterID
	sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 3}, {Name: "Plains", Count: 8}}, func(tbl *kop.Table, _ map[string]CardID) {
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
		creature = kop.FilterID(len(tbl.Filters) - 1)
	})
	fc, pl := id["Valley Floodcaller"], id["Plains"]
	opp := sc.perm(fc, 1)
	first := sc.perm(fc, 0)
	land := sc.perm(pl, 0)
	second := sc.perm(fc, 0)
	sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0}
	sc.s.g.nrem = 0
	in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtRememberAll)), F: creature}
	if !sc.s.exec(sc.k, &in) {
		t.Fatal("the remember op suspended")
	}
	want := []Obj{first, second, opp}
	got := sc.s.g.rem[:sc.s.g.nrem]
	if len(got) != len(want) {
		t.Fatalf("remembered %v, want %v (the land %d stays out)", got, want, land)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("remembered %v, want %v", got, want)
		}
	}
}

// TestDestroyRememberLKI: Destroy RememberLKI$ without AlwaysRemember$
// lowers to OpDestroy C bit 1 (destroyRemember): a target the destroy puts
// into the graveyard joins the remembered set; an indestructible one stays
// on the battlefield and out of the set (gorge effDestroy remembers only a
// card that reached the graveyard).
func TestDestroyRememberLKI(t *testing.T) {
	for _, spared := range []bool{false, true} {
		sc := newScenTable(t, v.NewTable(), Config{NoElide: true})
		o := sc.perm(v.GloriousAnthem, 0)
		sl := sc.slot(o)
		if spared {
			sc.s.grantEOT(sl, kop.KwIndestructible)
			if sc.s.kw(sc.k, sl)&kop.KwIndestructible == 0 {
				t.Fatal("precondition: the grant did not make it indestructible")
			}
		}
		sc.s.res.e = stackEntry{kind: seSpell, ctrl: 0}
		sc.s.g.nrem = 0
		sc.s.destroyRemember(sc.k, sl, 2)
		if spared {
			if !sc.onBF(o) || sc.s.g.nrem != 0 {
				t.Fatalf("indestructible: on the battlefield %v, remembered %v", sc.onBF(o), sc.s.g.rem[:sc.s.g.nrem])
			}
			continue
		}
		if sc.s.objs[o].zone != kop.ZoneGraveyard || sc.s.g.nrem != 1 || sc.s.g.rem[0] != o {
			t.Fatalf("destroyed: zone %d, remembered %v, want [%d]", sc.s.objs[o].zone, sc.s.g.rem[:sc.s.g.nrem], o)
		}
	}
}
