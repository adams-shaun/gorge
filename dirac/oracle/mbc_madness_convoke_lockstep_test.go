package oracle

import "testing"

// CR 702.51a with CR 702.35a: Markov Baron has Convoke and Madness {2}{B}, and
// the convoke applies to the madness cast too. Discarded to Go Blank with every
// land tapped, its owner still affords the madness cost: Chromatic Lantern
// pays one mana and two untapped creatures convoke the rest. The kernel priced
// the madness cast without the tap-paid credit (offering only "no"), and once
// offered, paid it through the plain window without tapping the picks
// (dr-mbc-baba pair 51, decision 443).
func TestMBCMarkovBaronReprosLockstep(t *testing.T) {
	t.Parallel()
	corpusDirOrSkip(t) // skip with no corpus before the recording is read
	stdReproNamed(t, "madness_cast_convoke_cost")
}
