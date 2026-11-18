package oracle

import "testing"

// Jubilant Skybonder grants each flier its controller controls "Spells your
// opponents cast that target this creature cost {2} more to cast." Casting Fate
// Foretold, the opponent may still pick the Skybonder as its target (CR 601.2c
// precedes the cost total of CR 601.2f); the two engines once disagreed on
// whether that target is offered (decision 870, tail-09 pair 4).
func TestMBCJubilantSkybonderAuraTargetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "skybonder_granted_target_tax")
}
