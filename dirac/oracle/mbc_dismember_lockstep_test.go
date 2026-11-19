package oracle

import "testing"

// CR 107.4f / 601.2b: Dismember's {B/P} pip, announced with only Domesticated
// Watercourse ({T}: Add {U} or {B}) as a black source, offers both faces (pay
// {B}, pay 2 life). The kernel once offered only the life face (kfuzz
// tail-49-c000 pair 2, decision 74); the Taps-deferred source's colour face
// is counted by addPipTapsDefer.
func TestMBCDismemberReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_pip_face_from_dual_combo_land")
}

// CR 605.3a with CR 601.2g: Dismember's fallback payment window (pool [B,G],
// {1}{B}{B} owed, one {B} still lacking) listed Bog Initiate's "{1}: Add {B}."
// forever: the stall rule modelled the fee's generic part as paid from the
// mana no fixed pip needs (the {G}), but the live payment spends generic mana
// in C,W,U,B,R,G order (pay/solve.go), so it ate the owed {B} and the
// activation changed nothing (a livelock at decision 224, tail-49 pair 108).
// feeOnlyConversionStalls and windowFeeStalls pay the generic part in that
// order too.
func TestMBCDismemberFeeOnlyStallOrderLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "feeonly-stall-order")
}
