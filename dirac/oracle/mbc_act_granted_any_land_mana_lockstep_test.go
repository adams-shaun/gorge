package oracle

import "testing"

// CR 605.1a with CR 113.6: Dune Chanter gives every land its controller
// controls "{T}: Add one mana of any color", so two Forests and four Islands
// pay Wakeroot Elemental's {G}{G}{G}{G}{G}: the Forests' printed {G} and three
// Islands tapped for green through the granted ability. gorge offers the
// activation; the kernel's grant search once found no prefix and left it out
// (decision 465, tail-71-c000 pair 179, autopay).
func TestMBCWakerootElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_offer_funded_by_granted_any_land_mana")
}
