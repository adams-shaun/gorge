package kgen

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGuardListsManaGrants: AbManaGrants lists the cards whose Grant body is a
// mana ability, not the ones granting a non-mana ability.
func TestGuardListsManaGrants(t *testing.T) {
	lantern := built{def: kop.CardDef{Name: "Lantern", Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Grant: 2}},
		{Kind: kop.AbMana},
	}}}
	equipper := built{def: kop.CardDef{Name: "Equipper", Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Grant: 2}},
		{Kind: kop.AbActivated},
	}}}
	bs := []built{equipper, lantern}
	_, _, _, _, _, _, _, _, _, _, _, _, grants, _, _, _, _ := guardLists(bs, nil, nil)
	if want := []int{1}; !slices.Equal(grants, want) {
		t.Errorf("manaGrants %v, want %v", grants, want)
	}
}
