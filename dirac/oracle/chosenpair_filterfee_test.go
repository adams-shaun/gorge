package oracle

import "testing"

// Hidden Volcano's {4}{R}, {T}, sacrifice: Discover 4 beside Captivating Cave
// ({1}, {T}: one mana of any colour, the only red the board has) and Sunken
// Citadel (a land-only {C}{C} beside its {C}). gorge's prefix search activates
// every census gap in battlefield order, the Citadel's pair before the Cave,
// so the pair's restricted mana funds the Cave's fee and two ordinary lands
// the rest. The kernel left the Citadel untapped and tapped an Island for the
// fee (CR 601.2g, 602.2b, 106.6).
func TestChosenPairFundsAFilterFeeLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "chosen_pair_filter_fee", "std-lci", Config{})
}
