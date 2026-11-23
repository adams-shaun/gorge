package oracle

import "testing"

// Two lockstep games at land 10c3137042 part at `projection Stack[0].Kind`
// (decision 454): the ref engine has Stack[0] = Kind 2 with Source 12 and
// Ability 128 while the kernel has Stack[0] = Kind 1 with Source 0 and
// Ability 0 — the two resolved entries swapped. Blisterspit Gremlin is the
// most implicated card; the last recorded action before the split is
// activate(#0,-,0).
func TestMBCBlisterspitGremlinReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "projection_stack_kind_blisterspit_gremlin")
}
