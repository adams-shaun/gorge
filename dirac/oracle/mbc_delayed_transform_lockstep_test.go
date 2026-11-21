package oracle

import "testing"

// CR 400.7 and CR 701.28f: Archangel Avacyn's "transform Archangel Avacyn at the
// beginning of the next upkeep" delayed trigger was registered while she was on the
// battlefield. She was bounced by Gust of Wind and, with the trigger already on the
// stack in the next upkeep, flashed back in: the new permanent is a different object,
// so the trigger resolves without transforming it. Gorge flipped the recast Avacyn
// (Avacyn the Purifier, 6/5, and its damage trigger) where the kernel left her alone
// (decision 388 of 388, tail-55 pair 53, projection Cards[3].Face).
func TestMBCArchangelAvacynReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delayed_transform_after_bounce_and_recast")
}
