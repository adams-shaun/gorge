package oracle

import "testing"

// CR 601.2h, 605.3b: Incubation Druid's adapted tap makes three mana of one
// type a land could produce (GGG beside a Forest), so Circle of Dreams Druid
// ({G}{G}{G}) is castable from the Druid alone while a Humble Naturalist (any
// colour, creature spells only) sits untapped beside it. Gorge offers the cast
// through the census's prefix search; the kernel's per-tap view of the
// restricted unit once refuted it (tail-55 pair 31, decision 429).
func TestMBCCircleOfDreamsDruidReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_amount_beside_restricted_any_prefix_offer")
}
