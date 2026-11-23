package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLandGrantRestrictedAnyPotential: a land whose first ability is {C} and
// whose second is a restricted any-colour one (Abundant Countryside: "Spend
// this mana only to cast a creature spell"), made an Island, folds {C} and the
// intrinsic {U} into the trigger-cost over-bound (gorge potentialManaFor
// counts [U1 C1], CR 305.6). The any-colour ability adds nothing to a
// trigger's own cost, so its colours must not shadow the intrinsic {U}: the
// cumulative upkeep {2} of Survivor of the Unseen is offered "pay" (pool
// tail-28-c000 pair 9, decision 757).
func TestLandGrantRestrictedAnyPotential(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	island := -1
	for j, st := range tbl.Subtypes {
		if st == "Island" {
			island = j
		}
	}
	if island < 0 {
		island = len(tbl.Subtypes)
		tbl.Subtypes = append(tbl.Subtypes, "Island")
	}
	var sm kop.SubMask
	sm[island/64] |= 1 << (uint(island) % 64)
	tbl.Overrides = append(tbl.Overrides, kop.Override{AddTypes: kop.TLand, AddSub: sm})
	tbl.Filters = append(tbl.Filters, kop.Filter{}) // matches every permanent
	grant := CardID(len(tbl.Cards))
	land := grant + 1
	tap := kop.Cost{Flags: kop.CostTap}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Land Grant", Types: kop.TEnchantment, Abilities: []kop.Ability{{
			Kind: kop.AbStatic,
			Static: kop.Static{
				Kind:     kop.StatOverride,
				Filter:   kop.FilterID(len(tbl.Filters) - 1),
				Override: uint16(len(tbl.Overrides)),
			},
		}}},
		kop.CardDef{Name: "Test Countryside", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: tap, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, AFlags: kop.AfManaChoice, Cost: tap, ManaColors: kop.ColAny, ManaN: 1, ManaRestrict: kop.FilterID(len(tbl.Filters) - 1)},
		}},
	)
	sc := newScenTable(t, tbl, Config{})
	o := sc.perm(land, 0)
	g := sc.perm(grant, 0)
	sc.s.perms[sc.slot(g)].flags |= pfTapped // the override makes the grant card itself an Island: tapped, it is no source
	sl := sc.slot(o)
	sc.s.perms[sl].flags &^= pfTapped | pfSick
	if !sc.k.gen.blgOn {
		t.Fatal("precondition: blgOn unset: the grant carries no basic land type")
	}
	if m := sc.s.blgMask(sc.k, sl); m != kop.ColU {
		t.Fatalf("precondition: granted intrinsic colours %v, want {U}", m)
	}
	other := payDesc{class: pdOther, subBound: true}
	var av avail
	sc.s.availPotentialFor(sc.k, 0, &av, &other, false)
	c := cost(1, kop.U)
	if !av.affords(&c, pipMaskOf(&c), 0) {
		t.Errorf("the over-bound does not reach {1}{U} (total %d): the intrinsic {U} is missing beside {C}", av.total)
	}
	two := cost(2)
	if !av.affords(&two, pipMaskOf(&two), 0) {
		t.Errorf("the over-bound does not reach {2} (total %d)", av.total)
	}
}
