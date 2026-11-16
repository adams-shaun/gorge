package oracle

import "testing"

// CR 605.1a, CR 605.3a: an Eldrazi Spawn's sacrifice-only mana ability is a
// planner unit (a last-resort tier of its own payment, like a Treasure's),
// not a census gap. A land activation whose only source of its coloured pip is
// a {1},{T} any-colour filter (Artifact Unknown Shores) beside a Spawn is
// still offered by gorge's filter search, so the kernel must take the filter
// path there too. Memorial to Genius' {4}{U},{T}, sacrifice and Blighted
// Fen's {4}{B},{T}, sacrifice are the same shape (tail-10 pair 2695).
func TestMBCMemorialToGeniusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_only_mana_beside_filter_land_offers_activation")
}
