package oracle

import "testing"

// A card cast from exile under a MayPlayIgnoreColor$ grant (You Find Some
// Prisoners' Interrogate Them: Cathartic Pyre, {1}{R}, from an opponent's
// exile) is paid from the lands gorge's autopay taps: an Island for the
// generic and a Mountain for the {R}, not the two Islands the kernel's plan
// took (CR 609.4b lets either pay). The recording is kfuzz -synth dr-mbc-aaab
// over a You Find Some Prisoners focus, pair 66.
func TestAnyColorExileCastTapsTheCostsOwnColour(t *testing.T) {
	stdReproNamed(t, "exile_cast_any_color_taps_the_costs_own_colour")
}
