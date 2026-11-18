package oracle

import (
	"testing"
)

// CR 601.2g and 107.3: the lowered cast's witness prices gorge's cast census,
// which leaves a dynamic {T}, sacrifice source out (legal.go priorityOptions;
// planCastDynSacSkip). Shrine of Boundless Growth held three charge counters
// beside the eight lands an {X}{X}{X}{R}{R} Crackle with Power cast could tap,
// and the kernel's {X} witness priced the Shrine's {T}, sacrifice: Add {C} for
// each counter on the witness's avail (avail.dynSac): it announced X = 3 (11
// mana) where gorge's census prices X = 2 (the eight lands), and the payment
// planner, which leaves the Shrine untouched (planCastDynSacSkip), found no
// plan and panicked `kernel: autopay found no plan` at decision 363 of pool
// tail-34-c001 (pair 9, seed 834125), parting from gorge, which cast it at
// X = 2.
func TestMBCCrackleWithPowerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "x_witness_counts_dynamic_sac_source")
}
