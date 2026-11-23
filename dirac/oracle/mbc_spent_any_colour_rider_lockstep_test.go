package oracle

import "testing"

// CR 609.4b, 601.2g-h: Stolen Strategy's upkeep effect grants "you may cast
// that card and you may spend mana as though it were mana of any color to
// cast it", and its controller cast Leeching Bite ({1}{G}) from exile with
// Archway of Innovation, a Plains and Cryptolith Rite's creature grants on
// the board. Gorge's cast planner prices the cast under the rider (the
// folded {2} generic, PlanManaUnderRider), so it plans the payment and taps
// its sources at payment, after the targets. The kernel's mid-cast price
// (plannedUnderRider) read the printed cost: the record carrying the rider
// was already spent, and only Cryptolith Rite's granted abilities make {G},
// which the planner census excludes, so it lowered the cast and tapped the
// witness's sources at priority -- Archway of Innovation and a Plains tapped
// and {W}{U} floated before the target ask, while gorge's pool was still
// empty (decision 847 of tail-15 pair 12; projection Cards[41].Flags).
func TestMBCArchwayFlagsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spent_any_colour_rider_cast_lowered")
}
