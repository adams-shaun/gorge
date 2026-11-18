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
	// Decision 355 of tail-26-c000 (seed 834125): Spell Burst over Urabrask's
	// Anointer; the kernel offered an extra cast(Spell Burst,-,0).
	stdReproNamed(t, "spell_burst_x_cast_offer")
	// Decision 581 of tail-26-c000 (seed 977927, pair 28; CR 601.2b-c, 601.2h):
	// Watchful Blisterzoa (mana value 6) is the only target and five mana
	// pay no X=6, but Crossroads Candleguide's repeatable "{2}: Add one mana
	// of any color" leaves gorge's payment census non-relaxable, so no proof
	// refuses the plain cast and gorge offers it; the kernel priced that
	// ability as a once-only paid gap and left the plain cast out.
	stdReproNamed(t, "xbound_target_repeatable_fee_source_nonrelaxable")
}
