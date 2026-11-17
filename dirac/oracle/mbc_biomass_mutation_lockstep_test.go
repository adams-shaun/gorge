package oracle

import "testing"

// CR 107.3 with CR 601.2b: Biomass Mutation's X is announced as it is cast, and
// any X the caster can pay for is a legal choice. The recording reaches the
// cast with the mana for X=1; dirac offered only X=0. It was dealt from the
// tail-52 synthetic pool, but is recorded on pool custom: tail-52's membership
// moves with the corpus, and Blessed Respite (a deck card) later left it. The
// custom pool is the table the recording's own decks need, so it replays
// whatever the corpus does.
func TestMBCBiomassMutationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "biomass_mutation_x_one")
}
