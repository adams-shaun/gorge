package oracle

import "testing"

// CR 601.2f-h: Bone Shards ({B}, sacrifice a creature or discard a card) is on gorge's
// priority list when the only black mana is a Llanowar Envoy's {1}{G} filter (the
// Countryside's any-colour mana is for creature spells only); dirac withheld the cast
// (tail-28 pair 104 before MBC was split).
func TestMBCBoneShardsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "altadd_part_paid_by_filter_source")
}
