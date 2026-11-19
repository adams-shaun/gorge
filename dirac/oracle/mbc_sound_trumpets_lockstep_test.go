package oracle

import "testing"

// CR 608.2h and 202.3e: Sound the Trumpets counters a spell and, when that
// spell's mana value was 2 or less, recruits. Inside Information cast with
// X=4 had mana value 5 on the stack, so no recruit; the kernel read the value
// after the Counter had moved the spell off the stack, where X is 0, and read
// 1, recruiting (decision 354 of std-hob pair 22,
// projection Cards[41].Pos).
func TestMBCSoundTheTrumpetsXSpellReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sound_trumpets_counter_x_spell_mana_value")
}
