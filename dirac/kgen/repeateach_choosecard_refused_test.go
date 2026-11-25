package kgen

import "testing"

// The per-player RepeatEach ChooseCard cards that lockstep showed diverging
// stay refused by name until the divergence is fixed (see the lockstep-gaps
// ticket): Highcliff Felidar (Defined$ You over the greatest-power pick, an
// empty pick in gorge where the kernel offers a lesser creature). A lowering
// that diverges from gorge must not ship, so a card here lowering clean means
// the divergence was fixed and lockstepped: move it to TestRepeatEachSlice's
// list (Tariff, Discovery/Dispersal and Flotsam/Jetsam moved that way).
func TestRepeatEachChooseCardStillRefused(t *testing.T) {
	names := []string{"Highcliff Felidar"}
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
