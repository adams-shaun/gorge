package oracle

import "testing"

// A menu option the offer walk declined must not return through the script
// route. At decision 1149 of pool tail-26 (pair 35, surface autopay) p1's
// Wishful Merfolk activation {1}{U} is priced by neither the floating pool
// (empty) nor the planner (its census is unrelaxable: the only blue source is
// Henge of Ramos's repeatable net-negative fee gap "{2},{T}: one mana of any
// color", whose funding gorge's planner census withholds), so gorge's priority
// list carries no option for it. The kernel's activateOffers declined it the
// same way, but its script route's prefix search funds Henge's fee from the
// other taps and offered the activation anyway. gorge's menu arm never consults
// a prefix search for an activation, so the kernel over-offered (options
// options, lead gorge, implicates Wishful Merfolk).
func TestMBCWishfulMerfolkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "act_prefix_menu_absent")
}
