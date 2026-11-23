package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// anyColourFilterScen: Hidden Grotto ({T}: Add {C}; {1}, {T}: Add one mana of
// any color) with a StatTapExtra static on the same card, restricted to its
// controller's permanents. Hidden Grotto's filter is a plain Produced$ Any
// (ManaColors ColAny, no AfManaChoice, not ManaEach) beside a first ability
// that makes only {C}, so a reader that takes the carrier's type from
// manaPCAt sees {C} instead of the colour the filter picks.
func anyColourFilterScen(t *testing.T, flags kop.StaticFlags, a int16) (*scen, Slot) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Hidden Grotto", Count: 4}, {Name: "Forest", Count: 20}}
	tbl, ids, _, err := cardsettest.Set(t, "filterx").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range ids {
		id[tbl.Cards[c].Name] = c
	}
	g := id["Hidden Grotto"]
	tc := *tbl
	tc.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	cd := &tc.Cards[g]
	cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
	ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, A: a, B: 1, StFlags: flags}}
	ab.Static.Filter = kop.FilterID(len(tc.Filters))
	tc.Filters = append(append([]kop.Filter(nil), tc.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
	cd.Abilities = append(cd.Abilities, ab)
	sc := newScenTableLib(t, &tc, id["Forest"], Config{NoElide: true})
	return sc, sc.slot(sc.perm(g, 0))
}

// TestAnyColourFilterTapExtraIsNoFixedVector: a filter that produces one mana
// of any colour picks that colour after its tap (CR 605.3b, 106.12a), so a
// reflected carrier adds one mana of the pick, not the {C} of the card's free
// first ability; a {C}-only (StTapOnC) carrier does not reach the tap at all.
// filtExtra must price no fixed vector for either. Reading the first ability's
// mask instead made a {C} cost payable that gorge refuses (Hidden Grotto,
// Rumble Arena, Three Tree City).
func TestAnyColourFilterTapExtraIsNoFixedVector(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags kop.StaticFlags
		col   int16
	}{
		{"reflect", kop.StTapReflect, int16(kop.C)},
		{"on-C", kop.StTapOnC, int16(kop.C)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, sl := anyColourFilterScen(t, tc.flags, tc.col)
			if !sc.k.gen.anyTapExtra {
				t.Fatal("precondition: the table has no tap-extra carrier")
			}
			fa := sc.s.filtAb(sc.k, sl)
			if fa.ManaColors != kop.ColAny || fa.AFlags&kop.AfManaChoice != 0 || fa.ManaEach {
				t.Fatalf("precondition: filter colours %#x flags %#x each %v, want a plain Produced$ Any filter",
					fa.ManaColors, fa.AFlags, fa.ManaEach)
			}
			if fa.Cost.Flags&kop.CostTap == 0 {
				t.Fatal("precondition: the filter costs no {T}, so it taps nothing")
			}
			pc := sc.s.manaPCAt(sc.k, sl)
			if m := srcMask(pc); m != 1<<kop.C {
				t.Fatalf("precondition: the first mana ability's mask is %b, want the bare {C} the buggy read used", m)
			}
			xv := sc.s.filtExtra(sc.k, sl)
			if xv != ([kop.NumManaKinds]uint8{}) {
				t.Fatalf("filtExtra = %v: an any-colour filter picks its colour after the tap, so a %s carrier prices no fixed vector",
					xv, tc.name)
			}
		})
	}
}
