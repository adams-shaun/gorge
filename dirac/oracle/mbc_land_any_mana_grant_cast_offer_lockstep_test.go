package oracle

import "testing"

// A transformed Mystic Monstrosity grants every land its controller controls
// "{T}: Add one mana of any color", so a hand card costing only coloured pips
// ({R}{R}{R}, Mijae Djinn) is payable with three untapped lands that have
// their own printed abilities: CR 601.2g-h lets the payment window activate
// each land's granted ability beside its own, and gorge's cast-offer gate
// prices that window and offers the cast. The kernel's autopay gate withheld
// the plain cast because its PLANNER census holds printed and intrinsic
// abilities only (the V1 grant exclusion) and the printed sources alone
// cannot pay the red pips; the priority list gorge walks is the offer gate's,
// not the planner's, so the kernel offered one cast fewer (tail-75 pool,
// pair 1, seed 746980, decision 909).
func TestMBCMijaeDjinnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "land_any_mana_grant_cast_offer")
}
