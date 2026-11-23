package oracle

import "testing"

// Herald of Serra's echo cost {2}{W}{W} at its upkeep ask: gorge offers pay
// and decline where dirac offers decline only, because its unless pricing
// reached no mana filter (Unknown Shores' {1},{T}: any colour) and priced the
// cost unpayable (pool tail-34-c001, pair 29, decision 294; CR 605.3b, 118.12).
func TestMBCHeraldOfSerraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "echo_unless_filter_reach")
}
