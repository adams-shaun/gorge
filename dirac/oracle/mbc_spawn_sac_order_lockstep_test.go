package oracle

import "testing"

// CR 605.3b, 602.2b: under Path of Annihilation each Eldrazi Spawn has its
// printed sacrifice-only {C} beside the granted "{T}: Add one mana of any
// color", and Meatsqueak, Hoard Lord's "another creature dies" trigger stacks
// from the first sacrifice. gorge's prefix search branches a tapped Spawn's
// printed sacrifice where its object sits in the activation order, before the
// next Spawn's tap, so the line is tap, sacrifice, tap, sacrifice, then a pass
// to resolve the Food trigger before Baloth Gorger is cast. The kernel tried
// the sacrifice only after every other gap, stopped at tap, tap, and sacrificed
// during the payment, leaving the trigger above the cast (dr-mbc-babb pair 7,
// decision 318; Bottle-Cap Blast owed).
func TestMBCBottleCapBlastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_spawn_sac_before_next_gap_tap")
}
