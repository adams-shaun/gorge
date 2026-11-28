package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// equipShapeCards are the Equipment whose restricted equip (CR 702.6a) names
// a target narrower than a creature or one subtype you control: a
// Legendary, token or IsCommander property, or several subtype clauses.
var equipShapeCards = []string{
	"Blackblade Reforged", "Brotherhood Regalia", "Excalibur, Sword of Eden", "Robe of the Archmagi",
	"Team Pennant", "Unstable Molecule Suit", "Wrecking Ball Arm",
}

// TestEquipShapes: every card of the restricted-equip-filter group builds
// with no refusal, and Blackblade Reforged keeps both equip abilities, each an
// activated AfEquip Attach with its own cost and the restricted one with a
// target filter other than the plain one's.
func TestEquipShapes(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(equipShapeCards, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	all, bs, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	defs := map[string]*kop.CardDef{}
	for i, c := range all {
		defs[c.Name] = &bs[i].def
		if r := costProgCastRouteReason(c); r != "" {
			t.Errorf("%s refused: %s", c.Name, r)
		}
	}
	for _, n := range equipShapeCards {
		if defs[n] == nil {
			t.Errorf("%s not built", n)
		}
	}
	d := defs["Blackblade Reforged"]
	if d == nil {
		t.Fatal("Blackblade Reforged not built")
	}
	var eq []*kop.Ability
	for i := range d.Abilities {
		if a := &d.Abilities[i]; a.Kind == kop.AbActivated && a.AFlags&kop.AfEquip != 0 {
			eq = append(eq, a)
		}
	}
	if len(eq) != 2 {
		t.Fatalf("Blackblade Reforged has %d equip abilities, want 2", len(eq))
	}
	if g0, g1 := eq[0].Cost.Mana.Generic, eq[1].Cost.Mana.Generic; g0 != 3 || g1 != 7 {
		t.Errorf("equip costs {%d} and {%d}, want {3} and {7}", g0, g1)
	}
	if eq[0].NTargets != 1 || eq[1].NTargets != 1 || eq[0].Targets[0].Filter == eq[1].Targets[0].Filter {
		t.Errorf("equip targets %+v / %+v: want one each, the legendary one's filter distinct", eq[0].Targets[0], eq[1].Targets[0])
	}
	if d.EquipCost.Generic != 7 {
		t.Errorf("EquipCost {%d}, want the unrestricted {7}", d.EquipCost.Generic)
	}
}

// TestEquipShapeRefusalNamesTheProperty: a shape kgen still refuses names
// the property (a player alternative in the target filter here), not the
// generic "equip shape".
func TestEquipShapeRefusalNamesTheProperty(t *testing.T) {
	a := &lAbility{Kind: akActivated, Name: "Equip", Sorcery: true, Prog: []lInstr{{Op: "Attach"}},
		Targets: []lTarget{{F: &lFilter{Players: 1, Clauses: []lClause{{Types: tCreature, Ctrl: whoYou}}}}}}
	if got := equipShapeRefusal(a); got != "equip shape: target filter" {
		t.Errorf("refusal %q, want the target-filter reason", got)
	}
	a.Targets[0].F.Players = 0
	if got := equipShapeRefusal(a); got != "" {
		t.Errorf("creature-you-control equip refused: %q", got)
	}
}
