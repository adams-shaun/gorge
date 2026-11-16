package oracle

import "testing"

// The `projection Stack[N].Ability` class: gorge and the dirac kernel project
// different ability identities for a stack item. Recorded at wt
// cli-20261115T124339Z-8cb0afec (post-rebase) from the dr-mbc-aaaa evidence
// loop (pool run, seed 7244991853654461400, decision 166, implicating Carrier
// Thrall): cast Deadly Dispute, choose to sacrifice Carrier Thrall, and the
// next decision's projection of Stack[2].Ability reads gorge=128 kernel=129.
func TestMBCCompostReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "projection_stack_ability")
}
