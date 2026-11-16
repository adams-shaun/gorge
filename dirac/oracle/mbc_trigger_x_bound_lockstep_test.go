package oracle

import "testing"

// CR 106.1a with CR 601.2b: Decree of Justice's cycling trigger ("you may pay
// {X}") offers X no larger than the mana the controller's untapped sources can
// make. Quirion Elves' "one mana of the chosen color" ability is exact once its
// colour is chosen, so the Elves bound X by two (its {G} and its chosen colour),
// as dirac does. Gorge priced the Chosen production unbounded and offered
// X = 0..64 (decision 706 of 706, dr-mbc-bbaa pair 55).
func TestMBCRogueSPassageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_x_cost_chosen_colour_bound")
}
