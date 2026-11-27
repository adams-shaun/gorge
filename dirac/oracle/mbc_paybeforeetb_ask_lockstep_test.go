package oracle

import "testing"

// A parked pay-before-entering move (ReplPayBeforeETB, the effect path) asks
// its controller the may-election (raGenPayETB); the answer resumes the parked
// move through payETBResume. The recorded game's kernel
// used to deadlock: payETBResume left res.ask set, so the resume returned
// early and the next pose hit payETBOptions with the answer's pre state -- no
// case, zero options, "a decision with no options".
//
// The paid game (synth tail-73 pair 81, seed base 382407): one land eligible,
// the strict-supersets single card discards with no pick, and the card enters.
func TestMBCTail73AskPaidLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paybeforeetb_ask_paid")
}
