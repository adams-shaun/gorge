package oracle

import "testing"

// CR 605.3b, 601.2b: a trigger's "you may pay {X}" (Squealing Devil) offers every
// X the payer's sources can fund, and a mana ability with a literal fee and a bigger
// production (Ferrous Lake's "{1}, {T}: Add {U}{R}") funds one more than its fee. With
// the Lake, two Powerstones and three basics untapped the announcement runs X = 0..6;
// gorge priced the Lake out of its one-tap supply and stopped at 5 (decision 271).
func TestMBCSquealingDevilReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_pay_x_counts_fee_source_net")
}
