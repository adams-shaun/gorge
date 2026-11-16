package oracle

import "testing"

// CR 115.2 with Shanna's CantTarget (ValidSA$ Activated,Triggered, Activator$
// Player.Opponent): Oko, Thief of Crowns' -5 is an activated ability, and its
// second target ("target creature an opponent controls with power 3 or less")
// is chosen on the ExchangeControl sub-ability, so the opponent's Shanna, Sisay's
// Legacy (2/2) is no legal target for it. The kernel withheld her; gorge
// classified the DB sub-ability as neither Activated nor Triggered, failed open
// and offered her. The recording is the first divergence of the tail-53 evidence
// pool, pair 614, at decision 263 (options options, autopay, gorge leading).
func TestMBCShannaSisaySLegacyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cant_target_sub_ability_activated")
}
