package oracle

import "testing"

// Sage's Dousing counters Zephyr Scribe unless its controller pays {3}
// (tail-42-c000 pair 31, decision 250): both engines must offer the same
// pay/decline modes.
func TestMBCSageSDousingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sages_dousing_unless_cost_affordable")
}
