package oracle

import "testing"

// CR 106.1a with CR 601.2b: Decree of Justice's cycling trigger ("you may pay
// {X}") offers X no larger than the mana the controller's untapped sources can
// make. Sea Gate's one ability ("{U} or one mana of the chosen color") makes a
// colour that is chosen once the land is on the battlefield, so the bound reads
// it as a payment does (gorge's potentialManaFor with the trigger's own cost)
// and counts the gate by its two colours. The kernel priced the gate an open
// source and offered X = 0..64 where gorge offered 0..5 (decision 591 of 657,
// dr-mbc-bbaa pair 16, Reality Shift the last spell cast).
func TestMBCRealityShiftReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_x_gate_chosen_colour_bound")
}
