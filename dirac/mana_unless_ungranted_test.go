package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// manaUnlessUngrantedScen is a Forest and a Rhystic-Cave-shaped chooser ("{T}:
// choose a colour, add one mana of it unless any player pays {1}", unless
// toggling kop.Ability.ManaUnless) with a {1}{G} creature in hand and an
// opponent holding an untapped Mountain.
func manaUnlessUngrantedScen(t *testing.T, unless bool) (*scen, Obj, Obj) {
	t.Helper()
	tbl := v.NewTable()
	// A table with expressions is a generated one (prepareGen): the
	// colour-asking route (askColGap) exists only there, as it does in every
	// kgen table.
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExDevotionChosen})
	add := func(cd kop.CardDef) CardID {
		tbl.Cards = append(tbl.Cards, cd)
		return CardID(len(tbl.Cards) - 1)
	}
	chooser := add(kop.CardDef{Name: "Test Unless Land", Types: kop.TLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColAny, ManaN: 1, ManaAskColour: true, ManaUnless: unless}}})
	forest := add(kop.CardDef{Name: "Test Forest", Types: kop.TLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: 1 << kop.G, ManaN: 1}}})
	bears := add(kop.CardDef{Name: "Test Bears", Types: kop.TCreature, Power: 2, Toughness: 2,
		Cost: kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{kop.G: 1}}})
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true})
	fo := sc.perm(forest, 0)
	co := sc.perm(chooser, 0)
	sc.perm(v.Mountain, 1)
	sc.hand(bears, 0)
	return sc, fo, co
}

// TestManaUnlessCaveUngrantedWitnessExcludesIt: the ungranted {X} witness is
// gorge's printed-source census (paymentPlanManaUnitsOnlyCompute). A Cave's
// unless source is a planner-deferred source the census never lists, so the
// witness must not count it -- with or without the unless. availForMode's
// deferGap learned that in the same change (tapsDeferDead); this walk is the
// second reader of the same census.
func TestManaUnlessCaveUngrantedWitnessExcludesIt(t *testing.T) {
	for _, unless := range []bool{false, true} {
		sc, fo, co := manaUnlessUngrantedScen(t, unless)
		// Precondition: the unless source is exactly one of the sources the
		// planner defers as dead (askColUnlessDead), never the plain Forest.
		dead := sc.s.tapsDeferDead(sc.k, 0)
		if dead.has(sc.slot(co)) != unless || dead.has(sc.slot(fo)) {
			t.Fatalf("unless=%v: tapsDeferDead = %v, want only the chooser when unless", unless, dead)
		}
		var a avail
		sc.s.availUngranted(sc.k, 0, &a)
		if a.total != 1 || a.n != 1 {
			t.Errorf("unless=%v: availUngranted total=%d n=%d, want 1/1 (the Forest only)", unless, a.total, a.n)
		}
	}
}
