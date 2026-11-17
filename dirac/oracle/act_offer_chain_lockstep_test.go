package oracle

import "testing"

// An activation paid through a costed mana chain is offered by both engines:
// Sage of the Unknowable's restricted {C} ("to cast a colorless spell or to
// activate an ability") pays the fee of Ferrous Lake's {1},{T}: Add {U}{R}
// (CR 605.3a: a mana ability is an activated ability, and its cost may be
// paid with mana), and the {U}{R} pays the activation (the Food token's {2},
// Avarice Amulet's equip {2}). gorge listed the play and the kernel withheld
// it: its first-only filter's fee bound counted no restricted source of its
// own. Recorded as tail-17 pairs 248 and 246 (surface autopay), on a custom
// pool: tail-17's membership moves.
func TestTail17ActOfferChainLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"act_offer_costed_chain_food",
		"act_offer_costed_chain_equip",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
