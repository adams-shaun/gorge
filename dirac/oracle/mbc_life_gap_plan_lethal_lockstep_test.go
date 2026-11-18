package oracle

import "testing"

// A flashback cast at 3 life beside Treasonous Ogre ("Pay 3 life: Add {R}") and
// a Gold token: gorge's planner never plans a payment whose summed life reaches
// the payer's life (pay/search.go walk, CR 119.4), so it pays through the
// token's sacrifice, where the kernel's plan paid the Ogre's 3 life and died
// (pair 3, pool dr-mbc-bbbb, surface autopay).
func TestMBCLifeGapPlanLethalLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_gap_plan_lethal_gold_token")
}

// Rescind's cycling ({2}) beside an untapped Island and Treasonous Ogre at 3
// life: the only prefix that funds it pays the Ogre's 3 life, a branch that ends
// the game, which gorge's search drops (scriptSearch.visit), so it offers no
// cycling where the kernel's script search paid the life down to 0 and offered
// it (pair 3, decision 561, pool dr-mbc-bbbb, surface autopay).
func TestMBCLifeGapScriptLethalLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "life_gap_script_lethal_cycling")
}
