package oracle

import "testing"

// CR 508.1c with CR 613.4c: Mogg Jailer can't attack while the defender
// controls an untapped creature with power 2 or less, and that power is the
// derived one. The defender's 1/1 Frontline Strategist wears Gaea's Might
// (+2/+2 until end of turn, cast in Jailer's controller's upkeep), so it has
// power 3 and doesn't hold Jailer back. gorge once read the printed power 1
// and offered no attack where dirac offered Jailer (tail-33 autopay pair 63,
// decision 107).
func TestMBCGaeaSMightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cant_attack_unless_defender_reads_derived_power")
}
