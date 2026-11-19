package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSpellOnlyGrantNoNonSpellUnit: a granted mana ability whose spend
// restriction names spells only (Galazeth Prismari's "Artifacts you control
// have '{T}: Add one mana of any color. Spend this mana only to cast an
// instant or sorcery spell'") is no unit of nonSpellRestrAffords's census, the
// unless / ward fallback's (CR 106.6: mana restricted to casting spells pays
// neither). The helper's census reads the pdOther payment descriptor
// (canPayGen's shape, interp_ask.go), so the grant is filtered out and the
// unless cost a printed restricted source cannot fund stays unpayable.
func TestSpellOnlyGrantNoNonSpellUnit(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}}
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	artifacts := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant, Zone: kop.ZoneStack})
	spells := kop.FilterID(len(tbl.Filters) - 1)
	grantee := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		// no creature: an artifact activates its {T} while summoning sick
		// (CR 302.6), as the Crystal Slipper of the repro does
		Name: "Test Slipper", Types: kop.TArtifact,
	})
	grant := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Grant", Types: kop.TEnchantment,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 1, ManaRestrict: spells},
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: artifacts, Grant: 1}},
		},
	})
	// a printed restricted source whose restriction admits the non-spell
	// payment (AfManaNonSpell, the Powerstone's shape): it sets k.restrSrc and
	// opens restrSrcNone's class shortcut, so the fallback's restricted-source
	// walk runs and the census it copies pays -- as the repro's table does
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name: "Test Printed Restricted Source", Types: kop.TLand,
		Abilities: []kop.Ability{{
			Kind: kop.AbMana, AFlags: kop.AfManaNonSpell, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU, ManaN: 1, ManaRestrict: artifacts,
		}},
	})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(grant, 0)
	gr := sc.perm(grantee, 0)
	sl := sc.slot(gr)
	if len(sc.k.gen.grantPC) == 0 || !sc.s.grantMay(sc.k, sl) {
		t.Fatal("precondition: the creature is not under the grant")
	}
	gi := sc.s.grantIdx(sc.k, sl)
	if gi < 0 || sc.k.gen.grantRestr[gi] == 0 {
		t.Fatal("precondition: the grant is not spend-restricted")
	}
	if !sc.k.restrSrc {
		t.Fatal("precondition: the table has no restricted source, so no payment descriptor is read")
	}
	r := sc.k.gen.grantOf[gi]
	d := payDesc{class: pdOther}
	if sc.s.restrSrcAdmits(sc.k, 0, sl, &sc.k.t.Cards[r.card].Abilities[r.ab], &d) {
		t.Fatal("precondition: the pdOther payment may spend the spell-restricted grant")
	}
	if pc := sc.s.manaPCGen(sc.k, sl); pc.manaAb < 0 {
		t.Fatal("precondition: the granted ability is not the source's mana view")
	}
	c := kop.ManaCost{Generic: 1}
	if sc.s.nonSpellRestrAffords(sc.k, 0, &c) {
		t.Fatal("nonSpellRestrAffords pays {1} with the spell-restricted granted mana (CR 106.6: it pays no non-spell payment)")
	}
}
