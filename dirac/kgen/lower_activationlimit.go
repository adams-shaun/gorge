package kgen

// refuseOnceTwice refuses a card with two "activate only once" abilities
// (GameActivationLimit$ 1 or Exhaust$ True): gorge counts each ability's
// activations on its own (legal_loyalty.go activationUsedCount by ability
// index), the kernel keeps one per-permanent bit (pfOnceGame).
func (lc *lowerCtx) refuseOnceTwice() {
	n := 0
	for _, a := range lc.out.Abilities {
		if a.Limit == 1 {
			n++
		}
	}
	if n > 1 {
		lc.refuseNode("card", "two activate-only-once abilities (the kernel tracks one per permanent)")
	}
}
