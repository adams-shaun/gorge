package oracle

import "testing"

// Edgar Markov's {3}{R}{W}{B} cast from hand (pool tail-51, surface full, the
// recorded shrunk game; decision 322): the board holds Heap Gate ({1},{T}: add
// one mana of any color), Radiant Grove, three Plains and Treasonous Ogre
// ("Pay 3 life: Add {R}"). The cost is payable under CR 601.2h: Heap Gate's
// paid activation makes the {B} (its {1} fee from a Plains), and the Ogre's
// life-only activation makes the {R} -- a unit its untapped census still
// counts beside the floating one, and gorge's payment planner prices it from
// the potential bound (PotentialMana), its witness replay activating the Ogre
// as the plan's last resort (potentialWitnessReaches). The kernel's filter
// search (filtScript) never activates a lifeOnlyPain unit: registersGapOther
// excludes a sole life-only ability from the gapOther life gaps, so the life
// pass branched only on those and the cast stayed unpriced, and the kernel's
// priority menu dropped it.
func TestMBCEdgarMarkovReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_script_misses_sole_life_pain_unit")
}
