package oracle

import "testing"

// CR 303.4f: a non-cast Enchant:Player Aura (Curse of Stalked Prey) a
// battlefield move puts onto the battlefield has its controller choose what
// it enchants among the living seats. The kernel offers the seats as
// enchant(P0/-,0), enchant(P1/-,0); gorge's canon collapsed both player
// candidates to enchant(-,-,k) (the option index as the only carrier), so
// the two menus never matched and the pair diverged at the ask (kfuzz
// tail-28 pair 59, Worldgorger Dragon's leaves-battlefield return).
func TestMBCWorldgorgerDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "enchant_player_canon")
}
