package oracle

import "testing"

// CR 702.86a, 614.12: Unleash's counter election is an as-enters choice. gorge
// asks it while the creature spell is still resolving (the spell on the stack),
// like the colour and type elections, so the kernel must pose it before the
// spell enters rather than queue it after the entry (decision 523 of
// tail-35-c000, pair 0, Dead Reveler: ref stack, kernel battlefield).
func TestMBCUnleashEntryAskReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unleash_asked_with_spell_on_stack")
}
