package oracle

import "testing"

// Vault Skirge's {1}{B/P} parts gorge and the kernel on the options offered:
// gorge's cast planner prices the pip-free mana half {1} with a witness that
// taps Fetid Heath's free "{T}: Add {C}" -- the only {B} the CR 601.2g window
// can then make is Heath's "{W/B}, {T}: Add {W}{W}, {W}{B} or {B}{B}" fee
// ability, funded by Gate to the Citadel's {W} -- so the window the pips
// resolve in is left without a colour face and the cast stays withheld, while
// the kernel's cast census prices the fee chain and offers the cast (class
// `options options`, synth tail-01 pool tail-01-c000 pair 1, decision 395,
// seed 849488, surface autopay). The planner now re-plans the half with its
// own tapped sources held out, leaving the pip's colour producers to the
// window (engine/rules/payment_plan.go castPlanPhyrexianReplan), and the
// recording replays clean.
func TestMBCVaultSkirgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_cast_fee_half_witness")
}
