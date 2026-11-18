package oracle

import "testing"

// CR 601.2b-g, 605.1a: Portent of Betrayal ({3}{R}, target creature) beside a
// Viridescent Bog ({1},{T}: Add {B}{G}), a Mana Confluence and a Catalyst
// Elemental ("Sacrifice: Add {R}{R}") that is the only creature on the
// battlefield. gorge's prefix search withholds the play: the line that pays
// with the Elemental's mana sacrifices the spell's only target, and no line
// that spares it pays {3}{R} (tail-12 pair 158, decision 163 of 163). The
// kernel offered it.
func TestMBCPortentOfBetrayalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cast_filter_fee_sac_source_is_only_target")
}
