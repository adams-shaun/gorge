package oracle

import "testing"

// Tilling Treefolk's ETB trigger ("When Tilling Treefolk enters, you may
// return up to two target land cards from your graveyard to your hand") is a
// CR 603.5 optional trigger: it goes on the stack when it triggers regardless
// of the choice, made at resolution. The controller announced ZERO targets at
// placement (CR 603.3d/601.2c) and accepted the effect (CR 603.5) — after
// which gorge re-posed the zero-target election as a choosecard at decision
// 664, holding the ability on the stack (NStack ref=1 kernel=0) while the
// kernel resolved and popped it: tail-46-c005 pair 5 (seed 1, mulligans 0,
// full surface), class `projection NStack`.
func TestMBCTillingTreefolkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "tilling_treefolk_projected_nstack")
}
