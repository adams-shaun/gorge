package oracle

import "testing"

// CR 107.4f, CR 601.2g, CR 605.3a: Immolating Souleater's {R/P} activation,
// its pip answered with the colour face, opens gorge's manual window owing {R}.
// Holdout Settlement ("{T}: Add {C}" beside "{T}, Tap an untapped creature you
// control: Add one mana of any color") stays listed there, because gorge reads
// every mana ability the window offers (paymentSourceUseful) and the second one
// makes {R}; the window answers the source's first ability, so Holdout taps for
// {C} before Akoum Refuge taps for {R}, and the {C} floats. dirac's clamp read
// only the first ability's colour, skipped Holdout and left it untapped (tail-42
// pair 629, decision 232: Cards[].Flags and Pool differed). The clamp now keeps
// a second-ability tap-another source whose tap-another ability makes a colour
// the cost owes (windowTapXSecondUseful), and the recording replays clean.
func TestPhyrexianWindowFirstSource(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_window_first_source")
}
