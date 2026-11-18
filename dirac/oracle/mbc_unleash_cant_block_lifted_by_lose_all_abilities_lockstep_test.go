package oracle

import "testing"

// CR 702.86a with CR 613.1f: an Unleash creature can't block only while the
// Unleash keyword still generates its CantBlock static. Stasis Field
// ("Enchanted creature has base power and toughness 0/2, has defender, and
// loses all other abilities") removes the keyword, so its unleashed Thrill-Kill
// Assassin keeps the +1/+1 counter but may block. The kernel read the printed
// CardDef.Unleash flag past the loss and refused the block, so its blockers
// decision was pass-only while gorge offered the two blocks (dr-mbc pair 148 of
// pool tail-60, seed 3844, autopay).
func TestMBCRootwallaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unleash_cant_block_lifted_by_lose_all_abilities")
}
