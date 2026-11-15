package oracle

import "testing"

// CR 605.1a with CR 602.2b: Mardu Banner's {R}{W}{B}, {T}, sacrifice is paid
// beside Nature's Embrace on a Forest ("{T}: Add two mana of any one color")
// and a Mercadian Bazaar holding storage counters (a {T}, remove-counters
// mana ability). The Embrace's two black mana, a Plains and one counter of the
// Bazaar pay it; gorge's one prefix search activates the storage land beside
// the granted gap and offers the activation, where the kernel's grant search
// branched on the grants alone and dropped it (pair 635 of the evidence pool tail-66, decision 1952
// of 1952).
func TestMBCMarduBannerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "storage_battery_beside_grant_pays_activation")
}
