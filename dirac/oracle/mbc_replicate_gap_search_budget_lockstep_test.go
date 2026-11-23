package oracle

import "testing"

// Leap of Flame's replicated cast ({U}{R} plus its {U}{R} replicate payment)
// beside a Moat Piranhas wearing Paradise Mantle (a granted any-colour mana
// ability, a census gap) and a Henge of Ramos ("{2},{T}: Add one mana of any
// color", a fee gap) (pool tail-26, surface full, the recorded shrunk game):
// gorge's prefix search over the two gaps once spent its node budget on the
// fee branches and priced the replicated play census_incomplete, offering the
// plain cast alone at decision 562, while the kernel offered the replicated
// cast too. Both engines now offer the same options at that decision.
func TestMBCLeapOfFlameReprosLockstep(t *testing.T) {
	stdReproNamed(t, "replicate_gap_search_budget_overoffers")
}
