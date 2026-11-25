package kgen

import "testing"

// untapDenialForms: an Effect holding the targeted player's creatures and
// lands tapped through their next untap step (gorge's UntapDenial
// restriction, ended by the Effect's own Phase$ Untap exile) lowers to one
// player-scoped OpSkipUntap. The exile trigger is Static$ True (CR 611.2a
// bookkeeping gorge resolves inline), so it lowers to nothing on the stack;
// TestUntapDenialEmitsNoDelayedTrigger pins the absence.
var untapDenialForms = []formCase{
	{card: "Exhaustion", form: "the Effect is one skip-untap of the target player's creatures and lands", lir: "SkipUntap obj=Target0 C=4"},
}

func TestUntapDenial(t *testing.T) { checkForms(t, untapDenialForms) }
