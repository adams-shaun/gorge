package oracle

import (
	"testing"
)

// CR 601.2b-c: Spell Burst targets a spell with mana value X, so countering the
// recorded stack's Slip Through Space (mana value 1) needs X = 1 and costs
// {1}{U}. p1 has only Paradise Mantle untapped (one mana of any colour), so
// gorge offers pass alone; the kernel once over-counted the any-colour source
// in the X-bound target sweep and offered cast(Spell Burst). Decision 593 of
// the recording.
func TestMBCSpellBurstReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spell_burst_priority")
}
