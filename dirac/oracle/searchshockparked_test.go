package oracle

import "testing"

// A search that puts several lands onto the battlefield (Genesis Device's
// sacrifice-and-search) suspends at a shock land's pay-life election: gorge
// has moved the cards up to that land and no further, the rest enter after the
// answer, and the shuffle follows the last. The kernel used to move every found
// card before it posed the election.
func TestSearchPutsLandsOntoBattlefieldAroundShockElection(t *testing.T) {
	t.Parallel()
	synthPairsMulligans(t, "std-trk", RunConfig{Config: Config{GorgeAutopaySurface: true}}, 0, []int{128}, nil)
}
