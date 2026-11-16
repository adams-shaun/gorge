package oracle

import "testing"

// Geralf's Messenger's cast carries improvise beside Inspiring Statuary, and
// Darkwater Catacombs ({1}, {T}: Add {U}{B}) is its only census gap. gorge's
// planner plans a plain improvise cast at the credited price (payment_plan.go:
// improvise alone is no shape:contribution), so the credited census misses
// {B}{B}{B} and the prefix search funds the catacombs' fee from a Swamp: the
// cast is offered. The kernel's script route skipped every contribution cast,
// so its gaps walk listed the cast but no search ran (decision 389, tail-08
// pair 541, mulligans 0, seed 1).
func TestMBCImproviseFilterFeeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "improvise_filter_fee")
}
