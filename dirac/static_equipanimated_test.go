package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEquipAnimEarthbendedEquipmentLand: Toph, the First Metalbender makes the
// controller's nontoken artifacts lands, so an Equipment can be the land an
// earthbend animates, and a creature Equipment is unattached (CR 301.5c,
// 704.5p). The Equipment arm of sba must be reachable on a table that has an
// Equip ability, an earthbend and such a static, and stay off when either the
// earthbend or the land-making static is missing.
func TestEquipAnimEarthbendedEquipmentLand(t *testing.T) {
	build := func(equip, bend, landStatic bool) *kop.Table {
		tbl := &kop.Table{
			Filters:   []kop.Filter{{}, {Types: kop.TArtifact}},
			Overrides: []kop.Override{{HasTypes: true, AddTypes: kop.TLand}},
		}
		var eq, bd, st kop.CardDef
		eq.Name = "Test Staff"
		if equip {
			eq.Abilities = []kop.Ability{{Kind: kop.AbActivated, AFlags: kop.AfEquip}}
		}
		bd.Name = "Test Bender"
		if bend {
			bd.Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{Op: kop.OpEarthbend}}}}
		}
		st.Name = "Test Lands"
		if landStatic {
			st.Abilities = []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride, Override: 1, Filter: 1}}}
		}
		tbl.Cards = []kop.CardDef{eq, bd, st}
		return tbl
	}
	for _, tc := range []struct {
		name                    string
		equip, bend, landStatic bool
		want                    bool
	}{
		{"all three", true, true, true, true},
		{"no earthbend", true, false, true, false},
		{"no land-making static", true, true, false, false},
		{"no equip", false, true, true, false},
	} {
		var g genPrep
		prepEquipAnim(build(tc.equip, tc.bend, tc.landStatic), &g)
		if g.equipAnim != tc.want {
			t.Errorf("%s: equipAnim = %v, want %v", tc.name, g.equipAnim, tc.want)
		}
	}
}

// TestEquipAnimEquipmentWithoutEquipAbility: an Equipment can be attached
// without ever having an Equip ability — Cryptic Coat's own enters trigger
// cloaks itself and attaches it — so the Equipment card's kop.TEquipment type
// alone arms the gate (CR 704.5p). The animator is Behind-the-Mask-shaped: an
// override that adds the Creature type on another object (Target0).
func TestEquipAnimEquipmentWithoutEquipAbility(t *testing.T) {
	build := func(animate bool) *kop.Table {
		tbl := &kop.Table{
			Overrides: []kop.Override{{HasTypes: true, AddTypes: kop.TCreature}},
		}
		coat := kop.CardDef{
			Name:  "Test Coat",
			Types: kop.TArtifact | kop.TEquipment,
		}
		anim := kop.CardDef{
			Name: "Test Animator",
			Abilities: []kop.Ability{{
				Kind: kop.AbSpell,
				Prog: []kop.Instr{{Op: kop.OpOverride, Obj: kop.RefTarget0, C: 0}},
			}},
		}
		if animate {
			tbl.Cards = []kop.CardDef{coat, anim}
		} else {
			tbl.Cards = []kop.CardDef{coat}
		}
		return tbl
	}
	// Precondition: the Equipment carries no ability at all, hence no
	// kop.AfEquip anywhere; the type bit alone is what the widened gate reads.
	coat := build(true).Cards[0]
	if coat.Name != "Test Coat" || coat.Types&kop.TEquipment == 0 {
		t.Fatalf("precondition broken: Cards[0] is not an Equipment (types %d)", coat.Types)
	}
	for _, a := range coat.Abilities {
		if a.AFlags&kop.AfEquip != 0 {
			t.Fatal("precondition broken: the Equipment has an Equip ability")
		}
	}
	for _, tc := range []struct {
		name    string
		animate bool
		want    bool
	}{
		{"animated", true, true},
		{"no animator", false, false},
	} {
		var g genPrep
		prepEquipAnim(build(tc.animate), &g)
		if g.equipAnim != tc.want {
			t.Errorf("%s: equipAnim = %v, want %v", tc.name, g.equipAnim, tc.want)
		}
	}
}
