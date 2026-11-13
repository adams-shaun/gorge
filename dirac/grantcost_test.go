package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// grantCostScen: Sanguine Syphoner equipped with Fishing Pole (its granted
// "{1}, {T}, Tap Fishing Pole: put a bait counter on Fishing Pole"), and
// Giada, Font of Hope as the only untapped mana source. Giada's {W} is
// restricted to Angel spells, so it cannot pay the Vampire's granted
// ability; an empty pool is the only thing that can.
func grantCostScen(t *testing.T) (sc *scen, src, pole, giada Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Fishing Pole", Count: 1}, {Name: "Sanguine Syphoner", Count: 1},
		{Name: "Giada, Font of Hope", Count: 1}, {Name: "Plains", Count: 10}}
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
	sc = newScenTable(t, tbl, Config{NoElide: true})
	src = sc.perm(id["Sanguine Syphoner"], 0)
	pole = sc.perm(id["Fishing Pole"], 0)
	sc.s.perms[sc.slot(pole)].attached = sc.slot(src)
	giada = sc.perm(id["Giada, Font of Hope"], 0)
	if !sc.s.windowRestr(sc.k, 0).has(sc.slot(giada)) {
		t.Fatal("Giada is not a window restricted source")
	}
	return sc, src, pole, giada
}

func grantedAct(d *Decision, src Obj) (Action, bool) {
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(src)) && a.Mode()&0x80 != 0 {
			return a, true
		}
	}
	var zero Action
	return zero, false
}

// TestGrantedAbilityRestrictedGap: a granted ability's mana cost belongs
// to its granting card, so the scripted gap walk must price Fishing Pole's
// {1} there, not assume a granted ability is free. Before, Giada's
// restricted {W} made the ability look payable at the offer, the script
// tapped Giada, and the payment found no plan (dirac/mana.go's
// "autopay found no plan", pair 2215).
func TestGrantedAbilityRestrictedGap(t *testing.T) {
	sc, src, pole, _ := grantCostScen(t)
	s := sc.s

	s.pl[0].pool[kop.W] = 1
	d := s.Step()
	a, ok := grantedAct(&d, src)
	if !ok {
		t.Fatalf("granted ability not offered with {1} in the pool: %v", d.Options[:d.N])
	}
	if a.Target() != TCode(uint8(pole)) {
		t.Fatalf("granted ability target %d, want the granting Equipment %d", a.Target(), pole)
	}
	s.Apply(a)
	s.Step()
	if s.perms[sc.slot(src)].flags&pfTapped == 0 || s.perms[sc.slot(pole)].flags&pfTapped == 0 {
		t.Fatalf("after the ability: source tapped=%v, Fishing Pole tapped=%v; want both",
			s.perms[sc.slot(src)].flags&pfTapped != 0, s.perms[sc.slot(pole)].flags&pfTapped != 0)
	}
	if s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool %v after the ability, want empty", s.pl[0].pool)
	}

	sc, src, pole, _ = grantCostScen(t)
	s = sc.s
	if !s.windowRestr(sc.k, 0).any() {
		t.Fatal("no restricted source to exercise")
	}
	d = s.Step()
	if a, ok := grantedAct(&d, src); ok {
		t.Fatalf("granted ability %d offered on Giada's restricted {W} alone; the pool cannot pay Fishing Pole's {1}", a.Mode())
	}
	ab := uint8(255)
	s.grantedAbilities(sc.k, sc.slot(src), func(c CardID, i uint8) {
		if c == s.objs[pole].card {
			ab = i
		}
	})
	if ab == 255 {
		t.Fatal("Fishing Pole grants no ability")
	}
	if s.FeatAbilityPoolPays(0, src, 0x80|ab, pole) {
		t.Fatal("FeatAbilityPoolPays says the empty pool pays the granted ability")
	}
	s.pl[0].pool[kop.W] = 1
	if !s.FeatAbilityPoolPays(0, src, 0x80|ab, pole) {
		t.Fatal("FeatAbilityPoolPays says the pool does not pay the granted {1}")
	}
}

// TestAttachedGrantNotGlobal: Fishing Pole's "equipped creature has ..." is
// an attached-only grant, the grantM walk's. It must not turn the table-wide
// globGrant walk on (it did, and on a whole-pool
// table every grantedAbilitiesN call then walked every static source), and
// the grant must still reach the equipped creature through grantM.
func TestAttachedGrantNotGlobal(t *testing.T) {
	sc, src, pole, _ := grantCostScen(t)
	s := sc.s
	if sc.k.gen.globGrant {
		t.Fatal("an attached-only grant turned globGrant on")
	}
	if c := s.objs[pole].card; sc.k.gen.has(c, gfGlobGrant) {
		t.Fatal("Fishing Pole carries gfGlobGrant")
	}
	n := 0
	s.grantedAbilities(sc.k, sc.slot(src), func(c CardID, _ uint8) {
		if c == s.objs[pole].card {
			n++
		}
	})
	if n != 1 {
		t.Fatalf("the equipped creature gains %d Fishing Pole abilities, want 1", n)
	}
	s.perms[sc.slot(pole)].attached = NoSlot
	n = 0
	s.grantedAbilities(sc.k, sc.slot(src), func(CardID, uint8) { n++ })
	if n != 0 {
		t.Fatalf("an unequipped creature gains %d abilities, want 0", n)
	}
}

// grantHostScen is a creature with two copies of a granting card attached, in that
// order, each with its own slot.
func grantHostScen(t *testing.T, set, grantor, creature string) (sc *scen, src, older, newer Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: grantor, Count: 2}, {Name: creature, Count: 1}, {Name: "Plains", Count: 10}}
	tbl, da, _, err := cardsettest.Set(t, set).Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTable(t, tbl, Config{NoElide: true})
	src = sc.perm(id[creature], 0)
	older, newer = sc.perm(id[grantor], 1), sc.perm(id[grantor], 1)
	sc.s.perms[sc.slot(older)].attached = sc.slot(src)
	sc.s.perms[sc.slot(newer)].attached = sc.slot(src)
	sc.s.pl[0].pool[kop.W] = 2
	return sc, src, older, newer
}

// TestSecondSugarCoatIsTheGrantor: a later Sugar Coat removes the ability the earlier
// one granted (CR 613.1f), so the one ability left is the later one's.
func TestSecondSugarCoatIsTheGrantor(t *testing.T) {
	sc, src, older, newer := grantHostScen(t, "blb", "Sugar Coat", "Starlit Soothsayer")
	if sc.s.layerTs(sc.slot(older)) >= sc.s.layerTs(sc.slot(newer)) {
		t.Fatal("the coats entered out of order")
	}
	d := sc.s.Step()
	a, ok := grantedAct(&d, src)
	if !ok {
		t.Fatalf("the coated creature offers no ability: %v", d.Options[:d.N])
	}
	if a.Target() != TCode(uint8(newer)) {
		t.Fatalf("granted by %d, want the later Sugar Coat %d, not %d", a.Target(), newer, older)
	}
}

// TestFirstListedPoleIsTheGrantor: of two Equipment giving one ability the older
// timestamp is listed first, whatever slots they hold (CR 613.7).
func TestFirstListedPoleIsTheGrantor(t *testing.T) {
	sc, src, low, high := grantHostScen(t, "fdn", "Fishing Pole", "Sanguine Syphoner")
	s := sc.s
	s.lstamp[sc.slot(low)], s.lstamp[sc.slot(high)] = s.lstamp[sc.slot(high)], s.lstamp[sc.slot(low)]
	d := s.Step()
	a, ok := grantedAct(&d, src)
	if !ok {
		t.Fatalf("the equipped creature offers no ability: %v", d.Options[:d.N])
	}
	if a.Target() != TCode(uint8(high)) {
		t.Fatalf("granted by %d, want the older Pole %d, not %d", a.Target(), high, low)
	}
}
