package oracle

import "testing"

// Bash to Bits' flashback ({4}{R}{R}) was offered by gorge as a cast from the
// graveyard in a priority decision where the kernel offered none: the cast is
// affordable without Energy Refractor (a Vivid Meadow charge counter, four
// tapped lands and an Orzhova {C}), but the kernel's offer gate requires
// filtScript to find a prefix script, and that search spent its whole node
// budget on Energy Refractor's repeatable fee-only activation ("{2}: Add one
// mana of any color", a 5-way colour branch per activation beside a full
// battlefield), starving the branch that pays without the filter (decision 393
// of 393, tail-02-c000 pair 55, seed 648290). The fee-stall clamps in
// mana_filterrepeat.go prune those branches.
func TestMBCBashToBitsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_cast_priced_over_ability_cost")
}
