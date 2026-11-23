package oracle

import "testing"

// TestMBCPhyrexianProwlerReprosLockstep: an unless tax ({2}) the payer can
// reach only through a deterministic self-sacrifice mana source whose
// production is an evaluated amount — Lotus Blossom's "{T}, Sacrifice Lotus
// Blossom: Add X mana of any one color, where X is the number of petal
// counters on Lotus Blossom" (CR 605.3a's payment window) — is offered the pay
// mode, not decline only. gorge's unless window census priced such a source at
// zero (its evaluated arm required a TapOnly cost), so Plague Fiend's "destroy
// that creature unless its controller pays {2}" offered gorge's payer only the
// decline while the kernel offered pay and decline (decision 790 of the pair;
// pool tail-74, pair 22, before the census admitted the class).
func TestMBCPhyrexianProwlerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_selfsac_amount")
}
