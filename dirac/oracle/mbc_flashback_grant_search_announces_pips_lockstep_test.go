package oracle

import "testing"

// CR 601.2b announces a spell's printed pip colours before its targets, and
// Elven Chorus's granted "{T}: Add one mana of any color" prices the {4}{B}
// flashback of Eviscerator's Insight beside its own Eldrazi Spawn tokens'
// printed "Sacrifice this creature: Add {C}" -- the census affords the cast,
// so gorge offers it. The kernel's budget-capped grant search must find that
// plan too, and its colour asks reorder their answers by the pips the play
// announces (scriptOrderAnswers): with the cast's pips untold, the search
// ordered no black source first, ran out of its node budget and the kernel
// dropped the option (decision 522 of a synth tail-51 game, gorge
// [pass cast(#8,-,2)] against kernel [pass]).
func TestMBCEvisceratorSInsightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_grant_search_announces_pips")
}
