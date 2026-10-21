// clone_etb_exceptions_test.go pins CR 707.9b/707.9d on the "enters as a
// copy" route (CR 614.1c): an ETB Clone body whose exception sets power and
// toughness (Quicksilver Gargantuan) or colour (Vesuvan Doppelganger) is
// offered as a copy election and the exception is part of the copy's
// copiable values, so a Clone of that copy has it too (CR 707.3's example).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestQuicksilverGargantuanEntersAsA77Copy is CR 707.9b/d: Quicksilver
// Gargantuan entering as a copy of Grizzly Bears is a 7/7 Grizzly Bears,
// and a Clone copying it is a 7/7 too (the final P/T is a copiable value).
func TestQuicksilverGargantuanEntersAsA77Copy(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1420, []string{"Quicksilver Gargantuan", "Clone", "Grizzly Bears"}, nil, nil)
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	qg := findCardObj(t, e, 0, "Quicksilver Gargantuan", state.ZHand)
	addMana(t, e, 0, "CCCCCUU")
	castCopyCard(t, e, qg, bears, 0)
	if hasNote(e, "unimplemented API Clone") {
		t.Fatal("Quicksilver Gargantuan's ETB copy took the unimplemented-API Clone fallback")
	}
	o := e.G.Obj(qg)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("Quicksilver Gargantuan did not enter as a Grizzly Bears copy: name %q", o.Face().Name)
	}
	if p, tg := e.Power(qg), e.Toughness(qg); p != 7 || tg != 7 {
		t.Fatalf("Quicksilver Gargantuan copy is %d/%d, want 7/7 (CR 707.9b)", p, tg)
	}
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, qg, 0)
	if p, tg := e.Power(clone), e.Toughness(clone); e.G.Obj(clone).Face().Name != "Grizzly Bears" || p != 7 || tg != 7 {
		t.Fatalf("Clone of the Gargantuan copy is %q %d/%d, want a 7/7 Grizzly Bears (CR 707.9b)", e.G.Obj(clone).Face().Name, p, tg)
	}
	replayCheck(t, e, cfg)
}

// hasUpkeepCopyTrigger reports whether o's face carries a linked Mode$ Phase
// trigger (Vesuvan Doppelganger's granted upkeep copy line).
func hasUpkeepCopyTrigger(o *state.Object) bool {
	for _, tr := range o.Face().Triggers {
		if tr.Mode == "Phase" && tr.Effect != nil && tr.Effect.API == "Clone" {
			return true
		}
	}
	return false
}

// TestVesuvanDoppelgangerEntersBlueWithItsUpkeepCopy is CR 707.9a/b and the
// CR 707.3 example: Vesuvan Doppelganger entering as a copy of Grizzly Bears
// is a blue Grizzly Bears with the upkeep copy trigger, and a Clone copying
// the Doppelganger is a blue Grizzly Bears with that trigger.
func TestVesuvanDoppelgangerEntersBlueWithItsUpkeepCopy(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1421, []string{"Vesuvan Doppelganger", "Clone", "Grizzly Bears"}, nil, nil)
	bears := findCardObj(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	vd := findCardObj(t, e, 0, "Vesuvan Doppelganger", state.ZHand)
	addMana(t, e, 0, "CCCUU")
	castCopyCard(t, e, vd, bears, 0)
	if hasNote(e, "unimplemented API Clone") {
		t.Fatal("Vesuvan Doppelganger's ETB copy took the unimplemented-API Clone fallback")
	}
	o := e.G.Obj(vd)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("Vesuvan Doppelganger did not enter as a Grizzly Bears copy: name %q", o.Face().Name)
	}
	if c := e.Colors(vd); c != "U" {
		t.Fatalf("Vesuvan Doppelganger copy has colours %q, want blue only (it doesn't copy the colour)", c)
	}
	if !hasUpkeepCopyTrigger(o) {
		t.Fatal("Vesuvan Doppelganger copy lacks its granted upkeep copy trigger (CR 707.9a)")
	}
	clone := findCardObj(t, e, 0, "Clone", state.ZHand)
	addMana(t, e, 0, "CCCU")
	castCopyCard(t, e, clone, vd, 0)
	co := e.G.Obj(clone)
	if co.Face().Name != "Grizzly Bears" || e.Colors(clone) != "U" || !hasUpkeepCopyTrigger(co) {
		t.Fatalf("Clone of the Doppelganger is %q colours %q trigger %v, want a blue Grizzly Bears with the upkeep trigger (CR 707.3)",
			co.Face().Name, e.Colors(clone), hasUpkeepCopyTrigger(co))
	}
	replayCheck(t, e, cfg)
}

// TestAutonSoldierCopyIsNotLegendary is CR 707.9b on the third carrier the
// widened ETB scope admits: Auton Soldier entering as a copy of a legendary
// creature is a non-legendary artifact copy with myriad.
func TestAutonSoldierCopyIsNotLegendary(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 1422, []string{"Auton Soldier", "Isamaru, Hound of Konda"}, nil, nil)
	dog := findCardObj(t, e, 0, "Isamaru, Hound of Konda", state.ZBattlefield)
	as := findCardObj(t, e, 0, "Auton Soldier", state.ZHand)
	addMana(t, e, 0, "CCCCUU")
	castCopyCard(t, e, as, dog, 0)
	o := e.G.Obj(as)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Isamaru, Hound of Konda" {
		t.Fatalf("Auton Soldier did not enter as an Isamaru copy (zone %v)", o.Zone)
	}
	if e.hasType(o, "Legendary") || !e.hasType(o, "Artifact") || !e.HasKeyword(as, "Myriad") {
		t.Fatalf("Auton Soldier copy: legendary %v artifact %v myriad %v, want false/true/true (CR 707.9b)",
			e.hasType(o, "Legendary"), e.hasType(o, "Artifact"), e.HasKeyword(as, "Myriad"))
	}
	if d := e.G.Obj(dog); d.Zone != state.ZBattlefield {
		t.Fatal("the legend rule put Isamaru into the graveyard: the copy is still legendary")
	}
	replayCheck(t, e, cfg)
}
