package oracle

import "testing"

// A may-play cast under the any-colour rider (Stolen Strategy / Dragon's
// Smile: Abundant Growth cast from exile, {G}) is planned by gorge on the
// folded cost {1}; that plan took Llanowar Wastes' {C}, which can pay no
// relaxed pip (CR 107.4c), so gorge dropped it and planned the colour pins in
// WUBRG order: W and U have no source, B is the Wastes' pain tap (one damage).
// The kernel tapped the folded plan's {C} and then a Forest for the pip,
// keeping its life and floating the {C} (tail-15 fx-wh pair 177, surface
// autopay, seed 158018).
func TestMBCPainCensusPaysLifeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rider_pinned_colour_plan_pays_pain_tap")
}
