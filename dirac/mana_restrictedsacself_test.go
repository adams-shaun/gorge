package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A restricted "{T}, Sacrifice this" unit (Jeweled Lotus's three mana of any
// one colour, spent only to cast your commander): the kernel plans it as a
// last-resort sacrificing unit like a Treasure's (dirac mana_restrictedcost.go,
// kop.RestrictedSacSelfShape), for exactly the payments its restriction
// admits (CR 106.6, 601.2g-h); what a payment leaves floats restricted. No
// object is a commander off the battlefield (kop.W7IsCommander), so the real
// restriction never admits a payment here: the scenarios swap it for a spell
// filter of their own, on a table copy.

// restrSacScen is the restrictedmana pool's table with Jeweled Lotus's
// restriction replaced by filter f (the spell types it admits: 0 any spell)
// and the named cards in the deck; the scenario starts empty.
func restrSacScen(t *testing.T, f kop.Filter, names ...string) (sc *scen, id map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 10}, {Name: "Jeweled Lotus", Count: 1}}
	set := cardsettest.Set(t, "restrictedmana")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the restrictedmana table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lotus := id["Jeweled Lotus"]
	tb := *tbl
	tb.Filters = append(slices.Clone(tbl.Filters), f)
	tb.Cards = slices.Clone(tbl.Cards)
	tb.Cards[lotus].Abilities = slices.Clone(tbl.Cards[lotus].Abilities)
	found := false
	for j := range tb.Cards[lotus].Abilities {
		if a := &tb.Cards[lotus].Abilities[j]; a.Kind == kop.AbMana {
			a.ManaRestrict = kop.FilterID(len(tb.Filters) - 1)
			found = true
		}
	}
	if !found {
		t.Fatal("Jeweled Lotus has no mana ability in the table")
	}
	sc = newScenTableLib(t, &tb, id["Plains"], Config{NoElide: true})
	return sc, id
}

// restrSacCast drives the offered cast of hand card o through its decisions
// to its resolution, reporting whether the offer listed it.
func restrSacCast(sc *scen, o Obj) bool {
	d := sc.s.Step()
	offered := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			sc.s.Apply(a)
			offered = true
			break
		}
	}
	if !offered {
		return false
	}
	for i := 0; i < 12; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			return false
		}
		sc.s.Apply(d.Options[0])
		if sc.s.objs[o].zone == kop.ZoneBattlefield {
			return true
		}
	}
	return false
}

// TestRestrictedSacSelfUnitAdmitsAndPays: Jeweled Lotus's unit pays a spell
// its restriction admits, sacrificing the Lotus, one chosen colour, and what
// the payment leaves floats restricted; it is no payment of a spell the
// restriction refuses, and it is the last resort beside a land.
func TestRestrictedSacSelfUnitAdmitsAndPays(t *testing.T) {
	anySpell := kop.Filter{}
	// (a) the unit alone pays Savannah Lions ({W}): the Lotus is sacrificed,
	// two of its three white float restricted.
	sc, id := restrSacScen(t, anySpell, "Savannah Lions")
	lotus := sc.perm(id["Jeweled Lotus"], 0)
	lions := sc.hand(id["Savannah Lions"], 0)
	if sc.s.objs[lotus].zone != kop.ZoneBattlefield {
		t.Fatal("(a) precondition: the Lotus is not on the battlefield")
	}
	if !restrSacCast(sc, lions) {
		t.Fatal("(a) the Lions withheld with only the Lotus's unit: want them offered")
	}
	if z := sc.s.objs[lotus].zone; z != kop.ZoneGraveyard {
		t.Fatalf("(a) the Lotus is in zone %d after its activation, want the graveyard (sacrificed)", z)
	}
	if sc.s.restrN(0) != 2 || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("(a) after the payment: restricted %v pool %v, want two mana floating restricted and an empty pool", sc.s.rb[0], sc.s.pl[0].pool)
	}

	// (b) a restriction the Lions fail (artifact spells only): withheld, the
	// Lotus untouched.
	scB, idB := restrSacScen(t, kop.Filter{Types: kop.TArtifact}, "Savannah Lions")
	lotusB := scB.perm(idB["Jeweled Lotus"], 0)
	lionsB := scB.hand(idB["Savannah Lions"], 0)
	if restrSacCast(scB, lionsB) {
		t.Fatal("(b) the Lions offered on an artifact-only unit: want them withheld")
	}
	if scB.s.objs[lotusB].zone != kop.ZoneBattlefield {
		t.Fatal("(b) the Lotus left the battlefield without paying")
	}

	// (c) beside a Plains the Lions are paid from the land, the Lotus stays.
	scC, idC := restrSacScen(t, anySpell, "Savannah Lions")
	lotusC := scC.perm(idC["Jeweled Lotus"], 0)
	scC.perm(idC["Plains"], 0)
	lionsC := scC.hand(idC["Savannah Lions"], 0)
	if !restrSacCast(scC, lionsC) {
		t.Fatal("(c) the Lions withheld beside a Plains: want them offered")
	}
	if scC.s.objs[lotusC].zone != kop.ZoneBattlefield || scC.s.restrN(0) != 0 {
		t.Fatalf("(c) the Lotus was sacrificed (zone %d) or floated restricted mana %v beside a Plains; it is the last resort", scC.s.objs[lotusC].zone, scC.s.rb[0])
	}
}
