package kgen

import "testing"

// untapDenialForms: an Effect holding the targeted player's creatures and
// lands tapped through their next untap step (gorge's UntapDenial
// restriction, ended by the Effect's own Phase$ Untap exile) lowers to one
// player-scoped OpSkipUntap, and the exile trigger a body-less delayed program (no emblem).
var untapDenialForms = []formCase{
	{card: "Exhaustion", form: "the Effect is one skip-untap of the target player's creatures and lands", lir: "SkipUntap obj=Target0 C=4"},
	{card: "Exhaustion", form: "the Effect's own untap trigger is a body-less delayed program at the target's upkeep", lir: "DelayedTrigger obj=Target0 C=1 CS=NextUpkeepOfPlayer"},
}

func TestUntapDenial(t *testing.T) { checkForms(t, untapDenialForms) }
