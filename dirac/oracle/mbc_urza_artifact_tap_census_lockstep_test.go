package oracle

import "testing"

// CR 601.2c, 601.2f, 601.2g: Urza, Lord High Artificer's "tap an untapped
// artifact: add {U}" can't be paid with no untapped artifact, and tapping only
// removes candidates, so the source is blocked for the whole payment window.
// A Chain of Vapor cast free from Urza's exile under Kasmina's {2} tax, with
// one untapped Plains, therefore has no affordable opposing-creature target:
// the kernel withheld them (decision 355 of dr-mbc-baaa pair 191), while the
// gorge side treated Urza as an unpriced source, left its window census
// incomplete and offered the taxed targets only to abort the cast at payment.
func TestMBCAvenMindcensorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_tap_artifact_source_without_artifact_leaves_window_census_complete")
}
