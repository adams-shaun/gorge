package oracle

import "testing"

// CR 605.3a with CR 107.1b: an unless cost's offer gate must price a FIXED
// two-colour mana production at its full value. Rune Snag's "counter target
// spell unless its controller pays {2}" is payable when the payer's only
// untapped mana besides a basic Island is Sunscorched Divide, whose "{1},
// {T}: Add {R}{W}" the Island's {U} funds; gorge's unless-cost census pinned
// the fixed {R}{W} production into two mutually exclusive one-colour
// alternatives, so the walk saw one unit of mana and offered decline only
// where the kernel offered pay (decision 297 of 297; tail-01 pair 143 before
// MBC was split).
func TestMBCRuneSnagReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_census_fixed_production")
}
