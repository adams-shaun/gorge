package oracle

import "testing"

// CR 118.12, 605.3b: Pact of Negation's upkeep "pay {3}{U}{U}" with no blue
// source but one Nomadic Elf ({1}{G}: add one mana of any color) makes one
// {U} per activation. The payer had no way to reach the second {U}, so gorge
// offers only the decline; dirac offered pay too (decision 343 of the
// tail-44 autopay pair 85).
func TestMBCPactOfNegationReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_any_colour_filter_activates_once")
}
