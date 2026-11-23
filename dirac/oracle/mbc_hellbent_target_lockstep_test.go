package oracle

import "testing"

// CR 601.2a, 601.2c: a spell cast from the hand has left it by the time its
// targets are chosen, so Demon's Jester's Hellbent (+2/+1 with no cards in
// hand) already makes it a 4/3 for Repel the Vile's "power 4 or greater" mode
// when the Repel is the last card in hand. Gorge offered the cast; the kernel
// judged the census with the Repel still in hand (dr tail-19 pair 21, decision
// 369).
func TestMBCRepelTheVileReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hellbent_target_after_cast_leaves_hand")
}
