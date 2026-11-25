package kgen

import "testing"

// The per-player RepeatEach ChooseCard cards that lockstep showed diverging
// stay refused by name until the divergence is fixed (see the lockstep-gaps
// ticket): Tariff (greatest-mana-value pick priced as an unless cost),
// Highcliff Felidar (Defined$ You over the greatest-power pick) and the split
// faces Discovery/Dispersal and Flotsam/Jetsam. A lowering that diverges from
// gorge must not ship, so a card here lowering clean means the divergence was
// fixed and lockstepped: move it to TestRepeatEachSlice's list.
func TestRepeatEachChooseCardStillRefused(t *testing.T) {
	names := []string{"Tariff", "Highcliff Felidar", "Discovery", "Flotsam"}
	lw := lowerNamedPool(t, names)
	refused := map[string]bool{}
	for _, c := range append(append([]*lCard(nil), lw.cards...), lw.tokens...) {
		if len(c.Refusals) != 0 {
			refused[c.Name] = true
		}
	}
	for _, n := range names {
		if !refused[n] {
			t.Errorf("%s lowers with no refusal: lockstep it (dirac/oracle repeateach_slice_lowers_test.go) and move it into TestRepeatEachSlice", n)
		}
	}
}
