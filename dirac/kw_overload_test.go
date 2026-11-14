package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestOverloadCastsAtItsCostAndSweeps: CR 702.96a. An overloaded cast is
// offered as its own hand cast, asks for no target, pays the overload cost
// and resolves the "each" program (the DestroyAll sweep here) rather than the
// printed spell ability.
func TestOverloadCastsAtItsCostAndSweeps(t *testing.T) {
	tbl := *testTable()
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	tbl.Conds = []kop.Cond{{}} // a non-nil Conds enables prepareGen
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	id := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Overload",
		Types: kop.TInstant,
		Cost:  kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{kop.R: 1}},
		// the printed spell is a no-op: only the overloaded program sweeps
		Abilities:    []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpNop}}}},
		HasOverload:  true,
		OverloadCost: kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.R: 1}},
		OverloadProg: []kop.Instr{{Op: kop.OpDestroyAll, F: creatures}},
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.lands(0, v.Mountain, 4)
	bear := sc.perm(v.GrizzlyBears, 1)
	giant := sc.perm(v.HillGiant, 1)
	o := sc.hand(id, 0)
	d := sc.s.Step()
	found := false
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == o && a.Target() == castOverloadTgt {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("overloaded cast not offered: kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	sc.do(t, AkCast, TCode(o), castOverloadTgt)
	// no target is announced: "target" reads "each" (CR 702.96b)
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	sc.s.Step() // the state-based actions
	if sc.onBF(bear) || sc.onBF(giant) {
		t.Fatalf("overload left bear=%v giant=%v on the battlefield", sc.onBF(bear), sc.onBF(giant))
	}
	if o := sc.s.objs[o]; o.zone != kop.ZoneGraveyard {
		t.Fatalf("spell zone %d after resolving, want graveyard", o.zone)
	}
}
