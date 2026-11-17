package oracle

import "testing"

// CR 605.1b with CR 305.7: Rikala, Homarid King holds three tide counters, so
// its gated TapsForMana trigger defers every Island of its controller in
// gorge's payment planner. One of those Islands wears Spreading Seas (a
// land-type setter: it loses its own abilities and has the intrinsic {T}: Add
// {U}). The kernel classed that Island deferred but left it out of the census
// gap (tapsGapActivatable read the setter's ability loss as a dead source), so
// the plain walk still counted it as an ordinary unit and offered Elven Riders
// and Karn, Legacy Reforged beside four Forests; gorge reaches those casts
// only through a scripted prefix, which the autopay surface drops
// (decision 373 of dr-mbc-abab pair 170).
func TestMBCElvenRidersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rikala_gated_island_under_land_type_setter_is_a_deferred_gap")
}
