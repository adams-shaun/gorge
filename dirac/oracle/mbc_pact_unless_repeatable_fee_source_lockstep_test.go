package oracle

import "testing"

// CR 605.3a, 602.2b: Nomadic Elf's "{1}{G}: Add one mana of any color" has no
// {T}, so a payment may activate it as often as it can fund the fee. Pact of
// Negation's upkeep "pay {3}{U}{U}" is therefore payable from nine green and
// white sources plus the Elf: two activations make the {U}{U} and the other
// five mana pay the rest (tail-44 pair 13, seed 933541, decision 606). The
// offer gate must price the Elf as many sources, not one, so the pay option
// is offered by both engines.
func TestMBCCopperCarapaceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pact_unless_repeatable_fee_source")
}
