package oracle

import "testing"

// CR 707.2 with CR 601.2c and 602.2: Mercurial Pretender enters as a copy of
// Flamestick Courier and activates the copied "{2}{R}, {T}: Target Goblin
// creature gets +2/+2 and has haste". Seat 0 holds an untapped Island, two
// Mountains, a Signpost Scarecrow and two Sandstone Needles, so the filter
// and the counter-cost gaps make the board a mix one. gorge asks the pump's
// target before any payment and leaves every source untouched at that ask.
// The kernel's plain-listing probe (plainListedPays) priced the activation
// from the printed Pretender's first ability (the granted {2}{U}{U} bounce)
// instead of the copied card's, found no plan, and ran the filter prefix
// before the target ask: it tapped a Needle (a depletion counter gone) and
// floated {R}{R}. The whole recorded prefix must be served, so a drifted
// recording fails.
func TestMBCMercurialPretenderPumpReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "copied_activation_priced_from_the_copied_card")
}
