package oracle

import "testing"

// CR 106.6, 601.2g, 605.3a: Vhal, Candlekeep Researcher's {C} can't be spent
// on a spell cast from hand, but it can pay the {2} of School of the Unseen's
// "{2}, {T}: Add one mana of any color", which makes the {U} Soulblade Djinn
// (3UU) needs. Gorge offers the cast (Island, School funded by Vhal, three
// Forests and Needleverge Pathway); the kernel once did not (decision 412 of
// tail-42-c000, seed 711696).
func TestMBCSoulbladeDjinnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_paid_by_restricted_mana_for_hand_spell")
}
