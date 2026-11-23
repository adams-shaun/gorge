package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The kernel's lowering of a spend restriction's filter words mirrors these
// gorge reads (dirac/kgen restrictedmana_filterwords_test.go): Activated.Equip
// is the object filter Card.Equip, an unknown word that matches nothing, and
// Spell.withDevoid reads the printed Devoid keyword.
func TestRestrictedManaFilterWordGorgeReads(t *testing.T) {
	g := state.NewGame([]string{"you"})
	add := func(script string) state.ObjID {
		card, diags := cards.ParseBytes("inline.txt", []byte(script))
		if len(diags) != 0 {
			t.Fatalf("parse: %v", diags)
		}
		card.Link()
		for _, f := range card.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(card, 0)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))
		return o.ID
	}
	equip := add("Name:Equip fixture\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:2\nOracle:test\n")
	devoid := add("Name:Devoid fixture\nManaCost:2 R\nTypes:Creature Eldrazi\nPT:2/2\nK:Devoid\nOracle:test\n")
	if !MatchesSpec(g, "Card.Equipment", equip, 0) {
		t.Fatal("precondition: the Equipment subtype does not match the equipment")
	}
	if MatchesSpec(g, "Card.Equip", equip, 0) {
		t.Error("Card.Equip matched the equipment: Activated.Equip is no never-match term")
	}
	if !MatchesSpec(g, "Card.withDevoid", devoid, 0) || MatchesSpec(g, "Card.withDevoid", equip, 0) {
		t.Error("Card.withDevoid does not read the printed Devoid keyword")
	}
}
