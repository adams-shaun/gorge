package oracle

import "testing"

// A resolution-generated continuous effect's affected set is determined when
// the effect begins and never changes (CR 611.2c). Temur Charm mode 3
// (Affected$ Creature.powerLE3, AddHiddenKeyword$ CARDNAME can't block.)
// resolves with seat 1's Broodhunter Wurm at power 4, so the Wurm is not in
// the set: Disfigure then shrinks it to power 2 and it must stay a legal
// blocker. gorge matched Affected$ at every layer read and the kernel read
// its emblem's filter at every ask, so each took the shrunk Wurm out of the
// block; now gorge freezes the matched ids (effects.lockEffectGrant) and
// kgen lowers the body to OpOverrideAll (lockedCantBlock), which sweeps once
// at resolution. The recording reaches the declare-blockers ask after the
// Disfigure; with either half reverted it diverges at decision 312.
func TestEffectGrantSetLockedAtBeginLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "effect_grant_set_locked_at_begin")
}
