package oracle

import "testing"

// TestMBCStormboundGeistReprosLockstep replays the recorded tail-70 game where
// Roaring Primadox's upkeep trigger bounces Stormbound Geist (the creature
// Endless Evil enchants) to hand before Endless Evil's own upkeep trigger
// resolves: the Aura falls off to the graveyard, and its trigger still resolves
// (CR 603.3c: a triggered ability on the stack is independent of its source),
// copying the geist as a 1/1 token — the "enchanted creature" referent is the
// source's last bearer, the LKI read CR 608.2h asks for. The kernel once
// resolved RefAttached for an off-battlefield source to nothing once the bearer
// had left too, so the token never appeared (decision 317, projection NInst).
func TestMBCStormboundGeistReprosLockstep(t *testing.T) {
	stdReproNamed(t, "enchant_lastbearer_copy")
}
