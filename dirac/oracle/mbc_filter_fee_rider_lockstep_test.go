package oracle

import "testing"

// CR 605.3a, 601.2g: p0's Chorus of Might {3}{G} beside a Grove of the
// Burnwillows ({T}: {C}; {T}: {R} or {G}, each opponent gains 1 life) and
// H.E.R.B.I.E., Lovable Robot ({1}, {T}: one mana of any color) as the only
// green. gorge's prefix search hoists the needed {G} to the Grove's plain
// mana ask — a plain ask keeps the needed-colour reorder unless an option
// costs the payer life as it resolves, and the Grove's rider costs only its
// opponents — so it taps the Grove for {G} and its opponent gains 1 life.
// A kernel that dropped the reorder at the plain ask tapped the Grove for {C}
// instead and left the fee's {G} to the Robot's filter, so the Robot got
// tapped and no one gained life; the game split at decision 196
// (Cards[14].Flags, Life[1]). The recorded game now runs clean: replay holds
// gorge and kernel together (tail-04 pair 137, decision 196).
func TestHerbieFilterFeeBesideRiderGapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_beside_rider_source_gap")
}
