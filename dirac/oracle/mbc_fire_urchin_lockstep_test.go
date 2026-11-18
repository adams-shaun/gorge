package oracle

import "testing"

// CR 303.4f: an Aura put onto the battlefield without being cast chooses what
// it enchants as it enters, and a player is as legal a bearer as a permanent.
// With two players to pick from, gorge's "enchant" options carry the seat in
// Option.Player with no object; the canonical form dropped the seat, so the
// pair parted at the Aura's election while the kernel named P0 and P1
// (decision 681 of tail-28-c000 pair 93, Fire Urchin in the game).
func TestMBCFireUrchinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_enchants_player_as_enters")
}
