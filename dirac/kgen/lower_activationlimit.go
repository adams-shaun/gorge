package kgen

// numberOnceAbilities numbers a card's "activate only once" abilities
// (GameActivationLimit$ 1 or Exhaust$ True) in ability order. gorge counts
// each ability's activations on its own (legal_loyalty.go
// activationUsedCount by ability index); the kernel keeps the first in
// pfOnceGame and the second and third in the arena masks (mechOnce2/3), so
// a fourth has no carrier and is refused by name.
func (lc *lowerCtx) numberOnceAbilities() {
	n := 0
	for _, a := range lc.out.Abilities {
		if a.Limit != 1 {
			continue
		}
		n++
		if n > 3 {
			lc.refuseNode("card", "four activate-only-once abilities (the kernel tracks three per permanent)")
			return
		}
		a.Once = n
	}
}
