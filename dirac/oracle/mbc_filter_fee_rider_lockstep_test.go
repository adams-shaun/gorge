package oracle

import "testing"

// CR 605.3a, 601.2g: p0's Chorus of Might {3}{G} beside a Grove of the
// Burnwillows ({T}: {C}; {T}: {R} or {G}, each opponent gains 1 life) and
// H.E.R.B.I.E., Lovable Robot ({1}, {T}: one mana of any color) as the only
// green. gorge's prefix search pays the fee with the Grove's plain {C} and taps
// the Robot for {G}; the kernel tapped the Grove for {G} instead, so the
// Robot stayed untapped and the opponent gained a life (tail-04 pair 137,
// decision 196).
func TestHerbieFilterFeeBesideRiderGapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_beside_rider_source_gap")
}
