package oracle

import "testing"

// Hour of Revelation ({3}{W}{W}{W}) beside The World Tree (pool tail-08, surface
// full, seed 813294, recorded shrunk game; decision 683): p0 has nine lands, six
// untapped, and with six lands The World Tree lets every land tap for one mana of
// any colour (CR 601.2f, 106.1b), so the six pay the cost. gorge's prefix search
// over the nine census gaps once spent its 160-node budget (potentialScriptNodes)
// and left the cast census_incomplete, so only the kernel offered it. gorge now
// answers a mana grant's gate once per walk, the cast is priced and offered by both
// sides, and the recorded game replays clean.
func TestMBCHourOfRevelationReprosLockstep(t *testing.T) {
	stdReproNamed(t, "hour_of_revelation_reduce_cost_present")
}
