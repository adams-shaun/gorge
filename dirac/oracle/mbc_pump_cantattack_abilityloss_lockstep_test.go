package oracle

import "testing"

// TestMBCTail00ReprosLockstep replays the synthetic tail-00 game the evidence
// loop recorded: seat 1 activates Alchemist's Vial's "{1}, {T}, Sacrifice
// Alchemist's Vial: Target creature can't attack or block this turn" on its
// own 3/3 Chronomaton, then casts Snakeform on the same creature ("loses all
// abilities", Animate RemoveAllAbilities$ True). At the declare-attackers
// step the restriction must still stand: it is a game-rule continuous effect,
// not an ability of the object (CR 613.11 names the class, applied after all
// other continuous effects), so the later layer-6 removal cannot end it. The
// kernel keeps the restriction in its CantAttack override and offered only
// pass; gorge wiped the grant with the removal and offered
// attack(Chronomaton) (divergence at decision 181: decision kind, gorge
// attackers / kernel priority). gorge was wrong; the walk now keeps the
// granted restriction lines (engine/rules/chars/derive.go).
func TestMBCTail00ReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pump_cantattack_abilityloss")
}
