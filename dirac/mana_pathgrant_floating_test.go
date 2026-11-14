package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

var pathNames = map[CardID]string{} // card id -> name of the scenario built last

// pathGrantScen: Path of Annihilation (Eldrazi you control have "{T}: Add one
// mana of any color"), two Eldrazi Spawn (each "Sacrifice this creature: Add
// {C}" beside the granted tap), Gilanra, two Plains and Tyvar, the Pummeler.
func pathGrantScen(t *testing.T) (sc *scen, tyvar Obj, spawns []Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Path of Annihilation", Count: 1}, {Name: "Gilanra, Caller of Wirewood", Count: 1},
		{Name: "Tyvar, the Pummeler", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "mbc")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok {
			t.Fatalf("%s not in the mbc pool", e.Name)
		} else if !set.Supported()[i] {
			t.Fatalf("%s is not supported in the mbc pool", e.Name)
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
	for c := range tbl.Cards {
		if tbl.Cards[c].Name == "c_0_1_eldrazi_spawn_sac" {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	if _, ok := id["c_0_1_eldrazi_spawn_sac"]; !ok {
		t.Fatal("the Eldrazi Spawn token is not in the matchup table")
	}
	for n, c := range id {
		pathNames[c] = n
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Path of Annihilation"], 0)
	for i := 0; i < 2; i++ {
		o := sc.perm(id["c_0_1_eldrazi_spawn_sac"], 0)
		sc.s.objs[o].of |= ofToken
		spawns = append(spawns, o)
	}
	sc.perm(id["Gilanra, Caller of Wirewood"], 0)
	sc.perm(id["Plains"], 0)
	sc.perm(id["Plains"], 0)
	tyvar = sc.perm(id["Tyvar, the Pummeler"], 0)
	return sc, tyvar, spawns
}

// TestPathGrantedSpawnTapFloatsBeforeSacrificePaysAbility: under Path of
// Annihilation each Eldrazi Spawn has its printed sacrifice-only {C} and the
// granted "{T}: Add one mana of any color". Tyvar's {3}{G}{G} needs a second {G}
// that only a Spawn's granted tap makes, beside Gilanra's {G}, the other Spawn's
// {C} and two Plains. The prefix search must step the Spawn's granted tap (its own
// ability is sacrifice-only, so grantTapOK refused it): the plan then found only
// Gilanra, the payment saw {G} floating and no {G} source, and the kernel
// panicked "autopay found no plan" (pair 1242 of the old dr-mbc-ba split).
func TestPathGrantedSpawnTapFloatsBeforeSacrificePaysAbility(t *testing.T) {
	sc, tyvar, spawns := pathGrantScen(t)
	s := sc.s
	d := s.Step()
	var act Action
	found := false
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(tyvar)) && a.Mode() == 1 {
			act, found = a, true
		}
	}
	if !found {
		t.Fatalf("Tyvar's {3}{G}{G} ability is not offered: %v", d.Options[:d.N])
	}
	s.Apply(act)
	// the prefix: one Spawn sacrificed for {C}, the other tapped for {G}, Gilanra's {G}
	wantPool := [kop.NumManaKinds]uint8{}
	wantPool[kop.G], wantPool[kop.C] = 2, 1
	if s.pl[0].pool != wantPool {
		t.Fatalf("pool %v after the announcement, want %v ({G}{G} from the tapped Spawn and Gilanra, {C} from the sacrificed one)", s.pl[0].pool, wantPool)
	}
	if s.objs[spawns[0]].zone == kop.ZoneBattlefield || !sc.tapped(spawns[1]) {
		t.Errorf("first Spawn zone %d, second tapped %v: want the first sacrificed and the second tapped",
			s.objs[spawns[0]].zone, sc.tapped(spawns[1]))
	}
	s.Step() // the payment: no "autopay found no plan"
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the payment, want empty", s.pl[0].pool)
	}
	if s.sp == 0 {
		t.Errorf("Tyvar's ability is not on the stack")
	}
	for _, o := range plains(s) {
		if !sc.tapped(o) {
			t.Errorf("a Plains is untapped: the payment did not spend the lands")
		}
	}
}

func plains(s *State) (out []Obj) {
	for o := Obj(0); o < Obj(s.nobj); o++ {
		if s.objs[o].zone == kop.ZoneBattlefield && pathNames[s.objs[o].card] == "Plains" {
			out = append(out, o)
		}
	}
	return out
}
