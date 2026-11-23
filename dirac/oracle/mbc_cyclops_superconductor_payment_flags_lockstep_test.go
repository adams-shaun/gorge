package oracle

import (
	"testing"
)

// Cyclops Superconductor ({1}{U}{R}) is cast from Mountains, Karakas and
// Plaza of Heroes beside Crossroads Candleguide's "{2}: Add one mana of any
// color" (tail-26 deal, pair 29, seed 173303). gorge's scripted-prefix search
// (scriptSearch.feeOnlyAskPrune) drops the Candleguide's {U} answer while a
// free bare-tap source can make {U}, and counts Plaza's restricted "any color,
// legendary spells only" tap as that source, so the funding walks past
// Candleguide until Plaza is tapped: five sources, one {R} left floating.
// dirac's mirror (filtFreeTapColour) read only the census ability and took the
// Candleguide after three taps, tapping four lands and floating nothing.
// The recording is the scenario's reconstruction (the reporting seat's focus
// file is unrecoverable); it replays the whole cast and must stay in lockstep.
func TestMBCCyclopsSuperconductorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "cyclops_superconductor_payment_flags")
}
