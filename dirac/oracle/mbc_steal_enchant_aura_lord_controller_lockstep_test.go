package oracle

import "testing"

// CR 613.8a with CR 613.3 and CR 109.4 (control in layer 2, keywords in
// layer 6): a Steal Enchantment that steals a second Steal Enchantment moves
// that Aura first (CR 613.7a layer order), and the stolen Aura's own "you
// control enchanted permanent" then reads its NEW controller, moving the
// creature it holds too (CR 613.8a: the inner effect depends on the outer
// one, because the outer changes the inner's controller). The Primal Rage
// under the chain ends on the top stealer's side, and its "Creatures you
// control have trample" grants trample on that side only. The kernel once
// left the inner want stale, holding the creature on the old stealer's side
// (decision 391 of 391; dr-mbc-baab full, m1, pair 220).
func TestMBCStealEnchantAuraLordControllerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "steal_enchant_aura_lord_controller")
}
