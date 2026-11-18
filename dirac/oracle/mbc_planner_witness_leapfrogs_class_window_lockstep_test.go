package oracle

import "testing"

// The kernel's payment planner picked each step's unit as the globally lowest
// object id among all suffix-feasible units, so it leapt over a source the
// count vector's class limit forced gorge's witness to spend first and named a
// different sacrifice set for the same count vector (Gelatinous Genesis for
// X=4, funded by seven lands and Jaheira's treasures: gorge sacrifices the two
// latest treasures, the kernel two earlier ones — dr-mbc-aabb pair 21). The
// projected battlefield order split at Cards[57].Pos. gorge's witness rule is
// the spec: each step is the smallest source-ID step that still leaves every
// class enough later members (pay.materialize); the kernel's lexLeast must
// mirror it.
func TestMBCKethekCrucibleGoliathReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "planner_witness_leapfrogs_class_window")
}
