package oracle

import "testing"

// The kernel must not offer a play gorge's potential walk does not list.
// Jeweled Lotus ("{T}, Sacrifice: Add three mana of any one color. Spend this
// mana only to cast a commander spell") feeds gorge's PotentialMana bound --
// the walk's nil payment admits every restriction and marks the bound
// (potentialRestricted) -- so gorge prices each potential play against the
// bound rebuilt for that play's payment (potentialPoolFor: potentialManaFor
// with the descriptor), without the Lotus's mana (CR 106.6). Urza, Lord High
// Artificer's {5} ability ("Shuffle your library, then exile the top card")
// is not payable from the rebuilt bound (the untapped lands and the
// tap-another-artifact ability alone make four), so gorge's potential walk
// does not list it and its prefix search never runs for it. The kernel's
// script offers enumerated the play beside its open any-walk bound and its
// tap-another search found a chain that pays it, so the priority menu offered
// the activation gorge withholds. Recorded as tail-64 pair 2 (surface
// autopay, mulligans 1, seed 445821).
func TestMBCUrzaLordHighArtificerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "activated_tap_ability_still_offered")
}
