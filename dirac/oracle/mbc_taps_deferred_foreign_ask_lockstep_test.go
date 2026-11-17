package oracle

import "testing"

// Iron Fist's power-up ({7}{R}) was offered by the kernel on a board of seven
// lands and a Talisman of Unity under the opponent's two Relic Binds: the
// Talisman is the one source gorge's planner defers (its tap fires the
// enchanting Aura's Taps watcher), so the activation is payable only through
// gorge's prefix search, and that search dead-ends where the Aura's trigger is
// controlled by the opponent and poses a mode ask (potential_script.go visit:
// another player's non-priority ask ends the line). The recording diverged at
// decision 569: options options, gorge leading, autopay surface. It replays on a
// custom pool (a table built from its own decks), not tail-08.
func TestMBCIronFistHeroForHireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "taps_deferred_foreign_ask_power_up")
}
