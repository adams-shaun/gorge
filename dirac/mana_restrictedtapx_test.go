package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrictedTapXManaAb is the first mana ability of the card named name in the restrictedtapx
// fixture's table.
func restrictedTapXManaAb(t *testing.T, name string) *kop.Ability {
	t.Helper()
	deck := []cardset.Entry{{Name: name, Count: 1}, {Name: "Island", Count: 5}}
	tbl, da, _, err := cardsettest.Set(t, "restrictedtapx").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range da {
		if tbl.Cards[c].Name != name {
			continue
		}
		for j := range tbl.Cards[c].Abilities {
			if a := &tbl.Cards[c].Abilities[j]; a.Kind == kop.AbMana {
				return a
			}
		}
	}
	t.Fatalf("%s: no mana ability in the table", name)
	return nil
}

// TestRestrictedTapXOneShape: Grand Architect's "Tap an untapped blue creature:
// Add {C}{C}, spend only on artifact spells and artifact abilities" is the
// restricted tap-another shape (and a tap-another source and honoured as a
// card's first mana ability); Urza's unrestricted {U} (kop.TapXManaOne) and
// Mishra's Workshop's restricted {T} are not.
func TestRestrictedTapXOneShape(t *testing.T) {
	ga := restrictedTapXManaAb(t, "Grand Architect")
	if ga.ManaRestrict == 0 || ga.ManaRestrictAb == 0 || ga.Cost.Flags != kop.CostTapX {
		t.Fatalf("Grand Architect's mana ability %+v: want both restriction halves and a bare tap-another cost", ga)
	}
	if !kop.RestrictedTapXOneShape(ga) || !kop.TapXMana(ga) || kop.TapXManaOne(ga) || !kop.BoundedOnceManaAb(ga) {
		t.Errorf("Grand Architect: RestrictedTapXOneShape %v, TapXMana %v, TapXManaOne %v, BoundedOnceManaAb %v; want true true false true",
			kop.RestrictedTapXOneShape(ga), kop.TapXMana(ga), kop.TapXManaOne(ga), kop.BoundedOnceManaAb(ga))
	}
	urza := restrictedTapXManaAb(t, "Urza, Lord High Artificer")
	if !kop.TapXManaOne(urza) || urza.ManaRestrict != 0 {
		t.Fatalf("Urza's mana ability %+v: want the unrestricted TapXManaOne shape", urza)
	}
	if kop.RestrictedTapXOneShape(urza) {
		t.Error("Urza's unrestricted tap-another mana is no restricted tap-another shape")
	}
	ws := restrictedTapXManaAb(t, "Mishra's Workshop")
	if ws.ManaRestrict == 0 || ws.Cost.Flags != kop.CostTap {
		t.Fatalf("Mishra's Workshop's mana ability %+v: want a restricted {T}", ws)
	}
	if kop.RestrictedTapXOneShape(ws) || kop.TapXMana(ws) {
		t.Error("a restricted {T} source is no restricted tap-another shape")
	}
}
