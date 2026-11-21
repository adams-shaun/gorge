package oracle

import "testing"

// TestMBCWardenLevelupReprosLockstep replays the synthetic tail-73 game the
// evidence loop recorded: Warden of the First Tree's {3}{W/B}{W/B}{W/B}
// ability is paid through gorge's manual CR 601.2g window, which at Phyrexian
// Tower lists its "{T}, Sacrifice a creature: Add {B}{B}" first (gorge's
// owedAbilitiesFirst, engine/rules/mana_activation.go: a fixed {B} pip the
// floating pool still owes sorts the ability ahead of the printed-first
// "{T}: Add {C}", whose colourless slot no pip of the cost owes) and the
// answer takes it, sacrificing Warden itself. The kernel's window walk took
// the printed-first {C} instead: its windows never take a later
// tap-and-sacrifice ability beside a plain first one, so it paid from A-Base
// Camp, Shadowblood Ridge ({1}, {T}: Add {B}{R}) and the tower's {C}, left the
// {R} floating and never sacrificed (divergence at decision 832: projection
// Cards[0].Flags — the kernel's two lands tapped, its one {R} in the pool, and
// Warden still on the battlefield).
func TestMBCWardenLevelupReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "owed_sac_second_window_answer")
}
