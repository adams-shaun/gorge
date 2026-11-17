package oracle

import (
	"testing"
)

// CR 508.1 with CR 509: Rite of the Raging Storm's static forbids its
// Lightning Rager tokens from attacking their controller, so the summoning-sick
// token of the turn has no legal attack pair at all and gorge's askAttackers
// records the forced empty declaration without posing the attackers decision
// (engine/rules/combat.go). anyAttacker's attack-tax branch priced pairs for
// the table's Baird, Steward of Argive without the scoped-CantAttack gate the
// plain branch applies, so the kernel posed a vacuous attackers decision
// (decision 906 of 906; tail-18 pool pair 78).
func TestMBCRainOfBladesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "attack_tax_anyoffer_ignores_cantattack")
}
