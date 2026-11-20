package oracle

import "testing"

// The fee-only ask prune's second cut (dirac filtFeeColStalls's
// filtFreeTapColour, gorge scriptSearch.feeOnlyFreeTapColour) dropped a
// colour a "free bare-tap source" can make -- but the searched play's own
// {T} source is not free: its one tap pays the play itself. The Surgical
// Bay's "{1}{U}, {T}, Sacrifice" beside its own "{T}: Add {U}" and
// Crossroads Candleguide's "{2}: Add one mana of any color": the only
// bare-tap source of {U} is the Bay, which the play taps, so the colour ask
// must keep {U} and the kernel must price and offer the activation like
// gorge's plain run does. With the cut counting the Bay, the kernel's filter
// search found no script and withheld the offer (decision 917 of the
// recording; kfuzz -synth tail-26 pair 3, seed 606056, surface autopay).
func TestMBCTheSurgicalBayReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "fee_stall_free_tap_own_source")
}
