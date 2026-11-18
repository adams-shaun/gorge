package oracle

import "testing"

// CR 601.2b-g, 602.2: Ghostfire Blade's equip {3} is reduced by {2} only for a
// colorless creature, and the activator's untapped sources are a Swamp, a
// Tangled Islet and a Selesnya Signet ({1},{T}: add {G}{W}), which together
// make three mana. Gorge offers every creature the activator controls
// (Blind Zealot, Rumbling Baloth beside the colorless Ebony Rhino) and pays the
// pick through the manual window with the Signet; the kernel pruned the menu to
// the Rhino because its census left the Signet's fee ability out.
func TestMBCBlindZealotReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "equip_gap_funded_target_offered")
}
