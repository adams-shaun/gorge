package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMayPlayRaiseSpendsRestrictedManaFirst: a free may-play cast's raise is
// still a payment (CR 601.2f, 118.9d), so the restricted mana floating beside
// the caster's pool that this spell admits is spent before ordinary sources
// (CR 106.6, gorge pay.emitRestrictedManaSpend). Karn, Legacy Reforged's
// artifact-only {C} pays Thorn of Amethyst's {1}; the kernel must not tap the
// census source the plan would use when the restricted batch alone covers the
// raise and leave the batch floating for later steps.
func TestMayPlayRaiseSpendsRestrictedManaFirst(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Exprs = []kop.Expr{} // the gen prep holds the restriction registry (a kgen table carries its exprs)
	artifactRestriction := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact})

	spell := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Artifact Spell", Types: kop.TArtifact,
		Cost: kop.ManaCost{Generic: 1}})
	tax := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Tax", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbStatic,
			Static: kop.Static{Kind: kop.StatIncreaseCost, A: 1, StFlags: kop.StRaiseOpp}}}})
	restrictedSource := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Restricted Source", Types: kop.TLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColC, ManaN: 1, ManaRestrict: artifactRestriction}}})
	source := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Source", Types: kop.TLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap},
			ManaColors: kop.ColG, ManaN: 1}}})

	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s := sc.s
	sc.perm(tax, 1) // p1's tax raises p0's spells {1}
	restr := sc.perm(restrictedSource, 0)
	s.perms[sc.slot(restr)].flags |= pfTapped // its mana already floated
	src := sc.perm(source, 0)

	// The artifact-only {C} floats as its restriction's batch kind (the
	// table's restrF index plus one).
	var kind uint8
	if sc.k.gen != nil {
		for i, f := range sc.k.gen.restrF {
			if f == artifactRestriction {
				kind = uint8(i) + 1
				break
			}
		}
	}
	if kind == 0 {
		t.Fatal("precondition: the table's artifact restriction is not registered")
	}
	s.setRestr(0, kind, kop.C, 1)
	o := sc.hand(spell, 0)
	if m := s.restrSpendable(sc.k, 0, spell, s.restrOriginOfObj(o)); m == 0 {
		t.Fatalf("precondition: the floating restricted mana is not spendable on the artifact cast")
	}
	sc.kcastBuild(o, spell, 0)
	e := &s.bld().e
	e.flags |= sfFree | sfMayPlay
	cost := s.buildCost(sc.k)
	if cost != (kop.ManaCost{Generic: 1}) {
		t.Fatalf("precondition: the free may-play cast owes %+v, want the tax's {1}", cost)
	}
	s.finishBuild(sc.k)

	if s.rbAny(0) {
		t.Errorf("the restricted {C} still floats after paying the raise")
	}
	if s.perms[sc.slot(src)].flags&pfTapped != 0 {
		t.Errorf("the payment tapped the census source instead of spending the restricted mana first")
	}
	if s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Errorf("pool after the payment %v, want empty", s.pl[0].pool)
	}
}
