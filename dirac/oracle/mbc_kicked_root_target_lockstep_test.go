package oracle

import "testing"

// CR 601.2b, 601.2c: Tear Asunder kicked announces no artifact-or-enchantment target (its root
// target count reads Count$Kicked.0.1); only the kicked link's nonland permanent. Its sole
// artifact, Krark-Clan Ironworks, is the very source the kicked cast's mana comes from (sacrifice
// it for {C}{C}), so the plain cast's root-target census found nothing in the hypothetical walk
// and gorge withheld a kicked cast the kernel offered (tail-43 pair 118, decision 398).
func TestMBCTearAsunderKickedRootReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_cast_root_target_waived")
}
