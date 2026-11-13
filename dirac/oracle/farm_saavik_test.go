package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The std-trk farm game TRK-s481 xe:RAND:2:9 diverged at decision 330 on
// Cards[32].Zone with Saavik, Stoic Student implicated. Saavik, cast during p0's
// combat, resolved under Deanna Troi, Psychic Counsel ("other creatures you control
// enter with an additional +1/+1 counter") and Kira Nerys, Firebrand ("a creature
// you control enters tapped and attacking while you control an attacking creature").
// gorge asks the CR 616.1 order of the two, and the order decides the result: Kira's
// body is a ChangeZone gorge classes "other", which commutes with nothing, and once
// the counter body has been applied the creature has entered and Kira's no longer
// does (tapped and attacking only when Kira is picked first). The kernel applied
// both without asking. It cannot pose that ask for a spell's entry, so it stalls
// (StallEntryOrder), as for a tap beside an untap, and the repro replays to the
// declared stall instead of a divergence.
func TestSaavikBesideKiraAndDeannaStalls(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	res := replayReproRes(t, reg, set, "enter_attacking_beside_counter_body_asks_order", true)
	if res.Stall != KernelStallPrefix+"entryorder" {
		t.Fatalf("stall = %q, want the kernel's entryorder stall", res.Stall)
	}
}
