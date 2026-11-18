package oracle

import "testing"

// CR 702.35a with CR 601.2f-h: a discarded Markov Baron (madness {2}{B},
// convoke) whose owner controls no creature to convoke with and only two
// untapped mana sources (a Swamp and Chromatic Lantern) cannot pay the
// {2}{B}, so only "no" is offered. gorge's madness ask treated its convoke
// keyword as a credit that might still pay and offered "yes" too; the
// credit is nil when no creature can be tapped (dr-mbc-baba pair 33,
// decision 302).
func TestMBCMarkovBaronUnpayableMadnessReprosLockstep(t *testing.T) {
	t.Parallel()
	corpusDirOrSkip(t) // skip with no corpus before the recording is read
	stdReproNamed(t, "madness_cast_convoke_no_creature_unpayable")
}
