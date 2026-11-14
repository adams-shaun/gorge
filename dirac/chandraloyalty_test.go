package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestChandraRestrictedLoyaltyMana: Chandra, Chill of Compliance's mana +1
// (a loyalty ability, no mana ability: CR 605.1a) adds its {U} on
// resolution as restricted mana (CR 106.6; gorge's ManaAdd restriction):
// it floats in State.rb, not the pool, so it pays a noncreature spell
// (Unsummon) and withholds a creature (Healer's Hawk) -- the
// probe's seed 6 decision 130, where the kernel offered
// two casts gorge withheld.
func TestChandraRestrictedLoyaltyMana(t *testing.T) {
	deck := []cardset.Entry{{Name: "Chandra, Chill of Compliance", Count: 1}, {Name: "Unsummon", Count: 1},
		{Name: "Healer's Hawk", Count: 2}, {Name: "Island", Count: 16}}
	set := cardsettest.Set(t, "rememberwrites")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the rememberwrites table", e.Name)
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
	for _, surf := range []bool{false, true} {
		sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeAutopaySurface: surf})
		s := sc.s
		ch := sc.perm(id["Chandra, Chill of Compliance"], 0)
		sc.perm(id["Healer's Hawk"], 1) // Unsummon's target
		uns := sc.hand(id["Unsummon"], 0)
		hawk := sc.hand(id["Healer's Hawk"], 0)
		// abilities 0 Surveil +1, 1 Mana +1, 2 Tap -X, 3 Emblem -6
		act, d := MakeAction(AkActivate, TCode(ch), 0, 1), s.Step()
		offered := false
		for _, a := range d.Options[:d.N] {
			offered = offered || a == act
		}
		if !offered {
			t.Fatalf("surface %v: the mana +1 is not offered: %v", surf, d.Options[:d.N])
		}
		s.Apply(act)
		for i := 0; i < 2; i++ { // both players pass: the +1 resolves
			if d = s.Step(); d.Kind != DkPriority || s.sp == 0 {
				t.Fatalf("surface %v: decision kind %d, stack %d: want priority over the +1", surf, d.Kind, s.sp)
			}
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
		pl := &s.pl[0]
		if sc.s.restrN(0) != 1 || sc.s.restrOf(0, kop.U) != 1 || pl.pool != ([kop.NumManaKinds]uint8{}) {
			t.Fatalf("surface %v: after the +1: restricted %v pool %v, want one restricted {U} and an empty pool", surf, sc.s.rb[0], pl.pool)
		}
		d = s.Step()
		if d.Kind != DkPriority || d.Player != 0 {
			t.Fatalf("surface %v: decision kind %d player %d, want p0's priority", surf, d.Kind, d.Player)
		}
		if !castOffered(&d, uns) || castOffered(&d, hawk) {
			t.Fatalf("surface %v: Unsummon offered %v, Healer's Hawk %v; want the noncreature spell only", surf, castOffered(&d, uns), castOffered(&d, hawk))
		}
		if surf && (!s.FeatPoolPays(0, uns, 0) || s.FeatPoolPays(0, hawk, 0)) {
			t.Fatalf("FeatPoolPays: Unsummon %v, Healer's Hawk %v; want true, false", s.FeatPoolPays(0, uns, 0), s.FeatPoolPays(0, hawk, 0))
		}
	}
}
