package oracle

import "testing"

// Energy Flux's granted trigger on a Powerstone token asks "sacrifice this
// artifact unless you pay {2}" with the tokens' own {C}{C} untapped: the
// unless is payable (CR 106.6 leaves every non-spell payment free of a
// spell-only spend restriction), so gorge offers pay and decline. The kernel
// priced the {2} unpayable (its census drops a restricted source's ability
// and the exact gate read no restricted-source route) and offered decline
// only: decision 602 of 603, dr-mbc-aaab pair 79 ( gorge leads, options
// options: modes/mode of the unless ask).
func TestMBCPowerstoneUnlessReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "powerstone_unless_payable")
}
