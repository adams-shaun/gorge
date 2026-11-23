package oracle

import (
	"testing"
)

// A parked pay-before-entering move (ReplPayBeforeETB, the effect path) asks
// its controller the may-election (raGenPayETB); the answer resumes the parked
// move through payETBResume. Both answers are recorded games whose kernel
// used to deadlock: payETBResume left res.ask set, so the resume returned
// early and the next pose hit payETBOptions with the answer's pre state -- no
// case, zero options, "a decision with no options".
//
// The declined game (synth tail-73 pair 13, seed base 382397): two lands
// eligible, the election answered no, the Mox Diamond goes to its owner's
// graveyard (the replacement chain's ChangeZone-to-graveyard branch). The
// paid game (pair 81, seed base 382407): one land eligible, the strict-
// supersets single card discards with no pick, and the card enters.
func TestMBCTail73AskDeclinedLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paybeforeetb_ask_declined")
}

func TestMBCTail73AskPaidLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paybeforeetb_ask_paid")
}

// guard the preconditions the replays depend on: both recordings are
// non-stall-declared lockstep games on the synthetic tail-73 pool that end on
// the parked move's may-election answer (a last recorded action different
// between the two), so a kernel that never poses the ask, or a resume that
// deadlocks, is itself the failure.
func TestPayETBAskReproHeaders(t *testing.T) {
	t.Parallel()
	last := map[string]Canon{}
	for _, name := range []string{"paybeforeetb_ask_declined", "paybeforeetb_ask_paid"} {
		r, err := ReadRepro("testdata/repro/" + name + ".repro.json")
		if err != nil {
			t.Fatal(err)
		}
		if r.Stall {
			t.Fatalf("%s: precondition: the recording must not declare a stall", name)
		}
		if r.Pool != "tail-73" || r.Surface == "" || len(r.Actions) == 0 {
			t.Fatalf("%s: precondition: pool %q surface %q actions %d", name, r.Pool, r.Surface, len(r.Actions))
		}
		last[name] = r.Actions[len(r.Actions)-1]
	}
	if last["paybeforeetb_ask_declined"] == last["paybeforeetb_ask_paid"] {
		t.Fatal("precondition: the two recordings must end on different election answers")
	}
}
