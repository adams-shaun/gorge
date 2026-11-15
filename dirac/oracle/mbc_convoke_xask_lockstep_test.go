package oracle

import "testing"

// CR 601.2b with CR 702.51a: a cast announces its convoke picks before X, and a
// pick that pays a coloured pip leaves only the other pips and the generic for
// the lands. Braingeyser ({X}{U}{U}) cast with two Kraken-blue picks and one
// blue land among four untapped lands can pay X up to 4 (the land that pays {U}
// is the only one it needs; the generic pick pays one of X). The kernel priced
// the announced X against the cost before the picks, found {U}{U} unpayable
// from a single blue land, and offered only X=1 where gorge offers 1 to 4
// (decision 229 of 229 in the recorded game, seed 5 pair 46 of tail-71).
func TestMBCCandelabraOfTawnosReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "convoke_pip_pick_x_ask_ceiling")
}
