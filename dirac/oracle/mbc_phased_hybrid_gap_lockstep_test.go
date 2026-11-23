package oracle

import "testing"

// CR 605.3a, 601.2g: Atsushi, the Blazing Sky ({2}{R}{R}) is castable off one
// Mountain, three Forests, Mystic Gate and Energy Refractor: two Forests pay
// the Refractor's {2} fee for its {R}, the Mountain makes the other {R}. The
// plain prefix search prunes that line (the Mountain is a free {R} source, so
// the Refractor's {R} is "dominated") and ends short of its 160-node budget;
// gorge then runs the phased order anyway, because Mystic Gate's {W/U} fee is a
// hybrid-fee gap (phasedScope), and that run pays. The kernel followed the
// plain run with the phased one only when the plain run spent its whole
// budget, so it withheld the cast (tail-02-c000 pair 7, decision 236: only
// gorge offered cast(#1)).
func TestMBCAtsushiTheBlazingSkyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_hybrid_gap_beside_fee_only_filter")
}
