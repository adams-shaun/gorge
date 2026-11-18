package oracle

import "testing"

// Mindwrack Liege's "{U/R}{U/R}{U/R}{U/R}: put a blue or red creature card
// from your hand onto the battlefield" after a land drop, beside River of
// Tears ("{T}: Add {U}. If you played a land this turn, add {B} instead"):
// the land makes {B}, never {U} (CR 605.1a), so it can't fund a hybrid pip's
// blue face. gorge's window census priced River by its root {U} and the
// kernel's pip ask mirrored it, so both offered "pay U" for the fourth pip
// the activation could not pay; gorge then tapped River for {B} and aborted
// the activation (decision 764 of pair 9, pool tail-41). Both engines now
// offer the red face alone.
func TestMBCMindwrackLiegeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "conditional_mana_source_pays_hybrid_cost")
}
