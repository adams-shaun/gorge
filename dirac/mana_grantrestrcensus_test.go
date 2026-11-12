package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestGrantedRestrictedGrantKeepsChoiceSourceOutOfCensus: a source whose own
// ability is choice-shaped (any colour) beside a spend-restricted grant of a
// fixed {G} tap ability is not one of gorge's shared-census units even when
// the payment's descriptor admits the grant (here AfManaNonSpell admits the
// non-spell payment). Gorge's census withholds every RestrictValid$ ability
// (rules mana_available.go, payment_plan.go's restricted layer), so the plan
// lists the plain {G} source first although the granted one entered first
// (key 8's source order).
func TestGrantedRestrictedGrantKeepsChoiceSourceOutOfCensus(t *testing.T) {
	tbl := *v.Table
	// a generated table (prepareGen returns nil for a hand-written one)
	tbl.Conds = []kop.Cond{{}}
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	creatures := kop.FilterID(len(tbl.Filters) - 1)
	choiceCard := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Any Source", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{
			Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 1,
		}},
	})
	plainCard := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Green Source", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{
			Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1,
		}},
	})
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Printed Restricted Source", Types: kop.TLand,
		Abilities: []kop.Ability{{
			Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU, ManaN: 1, ManaRestrict: creatures,
		}},
	})
	grant := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Restricted Grant", Types: kop.TEnchantment,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, AFlags: kop.AfLatent | kop.AfManaNonSpell, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1, ManaRestrict: creatures},
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: creatures, Grant: 1}},
		},
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(grant, 0)
	anySrc := sc.perm(choiceCard, 0) // enters first
	plain := sc.perm(plainCard, 0)
	sl := sc.slot(anySrc)
	if pc := sc.s.manaPCGen(sc.k, sl); pc.manaCol != kop.ColAny {
		t.Fatalf("precondition: the source's own ability is %#x, not choice-shaped", pc.manaCol)
	}
	if len(sc.k.gen.grantPC) == 0 || !sc.s.grantMay(sc.k, sl) {
		t.Fatal("precondition: the source is not under the grant")
	}
	gi := sc.s.grantIdx(sc.k, sl)
	if gi < 0 || sc.k.gen.grantRestr[gi] == 0 {
		t.Fatal("precondition: the grant is not spend-restricted")
	}
	if !sc.k.restrSrc {
		t.Fatal("precondition: the table has no restricted source, so the plan reads no payment descriptor")
	}
	d := sc.s.payDescNow(NoObj)
	if !sc.s.restrSrcAdmits(sc.k, 0, sl, &sc.k.t.Cards[grant].Abilities[0], &d) {
		t.Fatalf("precondition: the payment descriptor (class %d) does not admit the grant", d.class)
	}
	var c kop.ManaCost
	c.Pips[kop.G] = 2
	var pk planner
	sc.s.tapNeed(&pk, 0, &c, 0, false)
	sc.s.plan(sc.k, 0, &pk, NoObj)
	if !pk.found || pk.nu != 2 {
		t.Fatalf("plan: found %v units %d, want the two one-mana sources", pk.found, pk.nu)
	}
	if pk.units[0] != sc.slot(plain) || pk.units[1] != sl {
		t.Fatalf("plan units %v (plain slot %d, granted source slot %d): the restricted grant's source joined the deterministic census instead of the appended restricted layer",
			pk.units[:pk.nu], sc.slot(plain), sl)
	}
}
