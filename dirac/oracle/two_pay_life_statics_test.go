package oracle

import "testing"

// TestTwoPayLifeStaticsReplayIsClean: Gwenom's play-from-the-top Effect twice
// (Peter Parker's Camera): the replay reaches the cast without the kernel
// stalling (StallPayLifeShared), agreeing with gorge's two alternatives.
func TestTwoPayLifeStaticsReplayIsClean(t *testing.T) {
	res := replayStdReproResult(t, synthReg(t), "std-spm", "two_pay_life_statics_one_alternative_each", true)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.StallUnmodelled || res.Stall != "" && res.Stall != "decisions" && res.Stall != "turns" {
		t.Fatalf("stopped: stall %q unmodelled %v", res.Stall, res.StallUnmodelled)
	}
}
