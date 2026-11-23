package oracle

import "testing"

// CR 106.1 and 605.3a: Incubation Druid with a +1/+1 counter adds three mana of
// ONE type that a land its controller controls could produce, and Cascade
// Bluffs' {U/R},{T} filter sits beside it in the same payment. Gnottvold
// Slumbermound's {3}{R}{G}{G},{T},Sacrifice activation floated the Druid's
// three mana for gorge as {R}{R}{R} (the colour ask answered R), while the
// kernel's filter search chained the Druid as a Combo Any source and floated
// {W}{W}{R}: the pool differed at decision 392 of pair 42 (tail-55, seed
// 497848, focus-per-deck 6).
func TestMBCIncubationDruidFilterAmountReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "incubation_druid_amount_pool")
}
