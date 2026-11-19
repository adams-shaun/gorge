package oracle

import "testing"

// Cabal Therapy is cast from hand naming Cranial Plating, goes to the
// graveyard, and is flashed back later. gorge kept the first resolution's
// ChosenName on the card and effNameCard returned early, so the flashback
// named nothing new and resolved at once; the kernel asked the name again
// (tail-44 pair 5, decision 232, Cards.Zone). A resolving instant or sorcery
// now names again (CR 400.7: the earlier name belonged to an earlier object).
func TestMBCCabalTherapyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "namecard_spell_recast_names_again")
}
