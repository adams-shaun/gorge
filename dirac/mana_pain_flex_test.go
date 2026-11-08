package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLethalPainlandFlexStaysItsOwnType: a lethal painland's normal {C} unit
// must not fold into an identical plain {C} source's type. Gorge's phase-2
// key 5 reads the source's FlexAll (the {C} and every colour its withheld
// pay-life alternative could make), so the painland's unit is its own type.
// Without the flex term in plan's type match the two sources share one type
// and the painland takes the plain source's flex of 1.
func TestLethalPainlandFlexStaysItsOwnType(t *testing.T) {
	set := cardsettest.Set(t, "town")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the town pool", name)
		}
		return CardID(set.PoolID(i))
	}
	plain := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Wastes", Types: kop.TLand, Abilities: []kop.Ability{tapMana(kop.ColC)}})
	sc := newScenTableLib(t, &tbl, id("Forest"), Config{})
	if !sc.k.painMana {
		t.Fatal("precondition: Starting Town's painful ability is not modelled (k.painMana)")
	}
	sc.perm(plain, 0)               // the plain {C} source, first by timestamp
	sc.perm(id("Starting Town"), 0) // the lethal painland after it
	sc.s.pl[0].life = 1
	var pk planner
	pk.need[kop.W], pk.needT = 1, 1
	pk.dry = true
	sc.s.plan(sc.k, 0, &pk, NoObj)
	if pk.nu != 2 {
		t.Fatalf("precondition: %d units, want the two sources", pk.nu)
	}
	if pk.nt != 2 {
		t.Fatalf("%d types, want the plain source and the painland apart", pk.nt)
	}
	pt, tt := pk.utyp[0], pk.utyp[1]
	if pt == tt {
		t.Fatalf("both sources share type %d", pt)
	}
	if got := pk.types[pt].flex; got != 1 {
		t.Errorf("plain {C} source flex %d, want 1", got)
	}
	if got := pk.types[tt].flex; got != 6 {
		t.Errorf("lethal painland flex %d, want 6 ({C} and every colour)", got)
	}
}
