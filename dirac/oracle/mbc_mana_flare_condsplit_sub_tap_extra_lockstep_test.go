package oracle

import "testing"

// CR 605.1a with CR 605.1b/106.12a: tapping River of Tears after its
// controller played a land adds {B} (the Condition-split sub's production,
// not the root's {U}), and Mana Flare adds one mana of any type the land
// produced -- another {B}. {W}{W} from the Plains beside them pays
// Necroplasm's {1}{B}{B}, so the cast is a legal priority option. gorge's
// potential bound folded the static TapsForMana triggers' extra only over
// the root's productions and priced {B} at one, dropping the cast while
// dirac offered it (decision 581; pool tail-41-c002 pair 3, seed 711696).
func TestMBCNecroplasmReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_flare_condsplit_sub_tap_extra")
}
