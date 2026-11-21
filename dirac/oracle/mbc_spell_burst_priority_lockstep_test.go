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
	// Decision 760 of tail-26-c000 (seed 434573, pair 11; CR 601.2b-c, 601.2h,
	// CR 106.1): Flowstone Crusher (mana value 4) is the only target and
	// four mana pay no {4}{U}; the same repeatable Candleguide activation
	// sits untapped on the caster's own board, so gorge's census is again
	// non-relaxable and the count proof xBoundTargetPayable now runs: the
	// announcement the pool plus every census unit's largest production
	// cannot fund is refused, as the kernel's exact per-tap sweep always did.
	stdReproNamed(t, "xbound_fee_gap")
	// Decision 370 of tail-26-s000 (seed 306680, pair 17): gorge offers
	// cast(Spell Burst,-,0) next to the buyback cast; the kernel only the latter.
	stdReproNamed(t, "spell_burst_buyback_x_zero_option")
}
