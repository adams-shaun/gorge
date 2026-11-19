package oracle

import "testing"

// CR 601.2b, 601.2g: Stolen by the Fae ("target creature with mana value X")
// beside a Wooded Bastion, a hybrid-fee filter land ({G/W}, {T}: add two mana
// of one of those colours). gorge's planner funds that fee from nothing but
// mana already floating, so the board's Island, Island and Bastion's {C} pay
// X = 1 at most and no creature has mana value 0 or 1: the cast is not
// offered. The kernel's X-bound sweep priced the filter's two mana in each of
// its colours beside the fee charged as generic and offered X = 4 for the
// mana value 4 Tui and La (decision 622 of tail-15-c002 pair 48).
func TestMBCStolenByTheFaeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "hybrid_filter_xbound_never_funded")
}
