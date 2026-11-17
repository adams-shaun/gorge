package oracle

import (
	"testing"
)

// CR 201.4: Cabal Therapist's "choose a nonland card name" reaches its NameCard
// only through the ImmediateTrigger's Execute$, not through an ability's
// SubAbility$ chain or a replacement, so the oracle's name-choice universe
// narrowing (NameUniverseFor) missed it: the gorge side posed the whole
// corpus' names where the kernel offers the matchup's own cards. The ticket's
// first divergence was the same ask naming A-Buy Your Silence, which only
// gorge offered (evidence run ev-tail-15-c007, land 81a6ccdb4b).
func TestMBCABuyYourSilenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "namechoice_universe")
}
