package oracle

import "testing"

// CR 601.2f with CR 601.2b-c: Aether Tradewinds cast free off Urza, Lord High
// Artificer's exile-and-play ability still pays Kasmina, Enigmatic Mentor's {2}
// tax once the target is chosen, and the caster's one untapped Swamp cannot pay
// it: Urza's "tap an untapped artifact: add {U}" has no artifact to tap. The
// target ask must withhold Kasmina's controller's creatures and planeswalkers,
// as the kernel does. gorge's mana-window census once counted the artifact-less
// Urza as an unpriced source and so fell back to offering every target (decision
// 515 of 515; dr-mbc-baaa pair 133).
func TestMBCGuttersnipeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tap_artifact_source_without_artifact")
}
