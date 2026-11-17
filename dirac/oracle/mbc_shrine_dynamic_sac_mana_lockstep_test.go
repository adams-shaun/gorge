package oracle

import "testing"

const shrineDynamicSacRepro = "shrine_dynamic_sac_mana_unpriced"

// CR 605.1a, 601.2g: Shrine of Boundless Growth's "{T}, Sacrifice it: Add {C}
// for each charge counter" is a mana ability whose sacrifice may be paid while
// paying a cost. gorge prices it as a last-resort sacrifice unit; the kernel's
// dynamic-amount variants (gen.go varPC) now hold the {T}, sacrifice cost for
// an activated-ability offer (avail.dynSac), so its autopay counts the source
// and offers the Equip {2} an untapped Sandstone Bridge and the Shrine pay
// (tail-01 pair 43).
func TestMBCOathkeeperTakenoSDaishoReprosLockstep(t *testing.T) {
	t.Parallel()
	if res := replayRawRepro(t, shrineDynamicSacRepro); res.Div != nil || res.StallUnmodelled {
		t.Fatalf("divergence at %d: %s %s %v (stall %q)", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall)
	}
}
