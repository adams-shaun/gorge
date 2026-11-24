package oracle

import "testing"

// The elided-mana mirror (elided.go) hands the kernel a span's activations, but
// a span in which a stack object resolved is not mirrored: gorge drains its
// trigger queue between a payment plan's activations, so a Taps trigger the
// plan's tap queued resolves inside the span (stack_resolve, then a 2-point
// life loss), and the kernel's own re-derivation replays that resolution.
// Mirroring the taps and the loss anyway counted the loss twice (kernel Life[1]
// 2 below gorge's at decision 751 and, in the short game, 113).
func TestMBCElidedMirrorTriggerLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"elided_mirror_taps_trigger_life_loss_not_doubled",
		"elided_mirror_taps_trigger_life_loss_short",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
