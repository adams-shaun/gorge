package oracle

import "testing"

// CR 601.2g-h, 702.34a: Eviscerator's Insight's flashback ({4}{B} plus "sacrifice an artifact or
// creature") beside Prismite ({2}: one mana of any colour), Treasonous Ogre (pay 3 life: {R}, no tap)
// and one untapped Mountain. gorge offered the cast and dirac did not (tail-51-c000 pair 101,
// decision 181 of 181).
func TestMBCEvisceratorSInsightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_sacrifice_beside_life_mana")
}
