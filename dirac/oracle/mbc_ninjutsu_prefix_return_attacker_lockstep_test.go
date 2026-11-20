package oracle

import "testing"

// CR 702.49a: Skullsnatcher's ninjutsu "{B}, Return an unblocked attacker you
// control to hand" was payable with Phyrexian Tower's "{T}, Sacrifice a
// creature: Add {B}{B}". The potential planner's script search activated the
// Tower by sacrificing the ONLY unblocked attacker, then priced the ninjutsu
// from the mana alone: its plan covers the mana half, so the consumed Return
// candidate went unread. The script then failed to replay, gorge's canonical
// priority list dropped the activation and the kernel's kept it (tail-73 pair
// 5, decision 425, surface full). The planner now confirms a plan whose cost
// has an unreserved candidate part against the offer gate.
func TestMBCSkullsnatcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ninjutsu_prefix_return_attacker")
}
