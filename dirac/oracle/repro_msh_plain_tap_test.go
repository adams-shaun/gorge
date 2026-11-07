package oracle

import "testing"

// A witness that tapped a restricted source's plain unit floats no restricted
// mana: the Hideout made its ordinary {C} beside two Swamps' {B}, and the {B}
// another source added is not the Hideout's Villain-only mana (CR 106.6,
// 601.2g). The game is the std-msh sweep's pair 240 vs 35 at seed
// 13056020192189756660.
func TestStdMshPlainTapFloatsNoRestrictedMana(t *testing.T) {
	t.Parallel()
	stdReprosLockstep(t, "std-msh", []stdRepro{
		{"plain_tap_floats_no_restricted_mana", false, false},
	})
}
