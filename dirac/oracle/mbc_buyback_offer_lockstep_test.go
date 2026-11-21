package oracle

import "testing"

// Buyback (CR 702.27a) is an optional additional cost, offered beside the plain
// cast when the spell's mana cost plus the buyback cost is payable. Imps' Taunt
// with five untapped lands: the buyback cast was offered by dirac but not by gorge
// (tail-54-c000 pair 26, decision 322).
func TestMBCImpsTauntReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "buyback_offer_gate")
}
