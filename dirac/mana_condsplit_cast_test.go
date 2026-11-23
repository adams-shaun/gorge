package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// condSplitCastScen puts p0 in its own main phase with an empty stack over a
// lucky Gemstone Caverns-shaped Condition-split land (root {T}: {C} gated EQ0,
// arm one mana of any colour gated GE1 on the luck counter: kop.Ability's
// SplitGate lowered form, dirac/kgen lower_condsplit.go), a Plains and a Mana
// Flare body (a StatTapExtra reflect carrier: one more mana of the type any
// tap produced, CR 106.12a), and returns the scen with a {2}{B}{B} sorcery in
// p0's hand.
func condSplitCastScen(t *testing.T) (*scen, Obj) {
	t.Helper()
	tbl := v.NewTable()
	tbl.Exprs = []kop.Expr{} // prepareGen must run: the split source's views are a gen fact
	// Mana Flare's body (the reflect carrier): any permanent its controller taps
	refl := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect}}
	refl.Static.Filter = kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
	add := func(name string, cd kop.CardDef) CardID {
		id := CardID(len(tbl.Cards))
		cd.Name = name
		tbl.Cards = append(tbl.Cards, cd)
		return id
	}
	cav := add("Test Lucky Caverns", kop.CardDef{
		Types: kop.TLand,
		Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColC,
			ManaN:      1,
			SplitGate:  kop.SplitGateCounter,
			SplitCtr:   kop.CtrLuck,
			SplitCols:  kop.ColAny,
		}},
	})
	flare := add("Test Mana Flare", kop.CardDef{Types: kop.TEnchantment, Abilities: []kop.Ability{refl}})
	sorc := add("Test BB Sorcery", kop.CardDef{
		Cost:      kop.ManaCost{Generic: 2, Pips: [kop.NumManaKinds]uint8{2: 2}}, // {2}{B}{B}
		Types:     kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell}},
	})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	cavObj := sc.perm(cav, 0)
	if !sc.s.perms[sc.slot(cavObj)].addCtrN(sc.s, int(kop.CtrLuck), 1) {
		t.Fatal("could not put the luck counter on the Caverns")
	}
	sc.perm(v.Plains, 0)
	sc.perm(flare, 0)
	o := sc.hand(sorc, 0)
	return sc, o
}

// TestCondSplitCastOfferedOverReflectPick: a lucky Gemstone Caverns beside Mana
// Flare funds {2}{B}{B} with a Plains (CR 605.1a, 106.12a, 601.2g): the
// Caverns' tap makes one mana of any colour and the reflection one more of that
// colour, the Plains' W and its reflection pay the {2}, so the hand-cast offer
// gate lists the cast. The split source joins the census through the Taps-
// deferred route (mana_condsplit.go), priced by the pick its arm asks.
func TestCondSplitCastOfferedOverReflectPick(t *testing.T) {
	sc, o := condSplitCastScen(t)
	if !sc.castOffered(0, o) {
		t.Fatal("the {2}{B}{B} sorcery is not offered over a lucky Condition-split land beside a reflect carrier")
	}
}
