package oracle

import "testing"

// CR 605.3a-b: a payment may chain mana abilities -- here the prefix search
// that prices a play the planner census cannot: Flooded Grove's "{G/U}, {T}:
// Add {G}{G}, {G}{U}, or {U}{U}" funds Nantuko Monastery's threshold animate
// ({G}{W}) when a Plains the same turn animated into an Island floats the
// {G/U} fee's {U} first. The search's twin cut collapsed that dual-type
// Plains into an identical-name untapped plain Plains (the ask offers one
// mana option a source, so the option labels are equal), whose subtree holds
// no {U} activation; the chain died unfunded and the play was withheld both
// sides' plain census -- but the kernel's mirror, which fails the twin open
// on the type-override difference, kept offering it. The recording is the
// last decision of its game (1410 of 1410): the fee funds only from that
// Island side, and the collapse hid it.
func TestMBCNantukoMonasteryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "twin_cut_dual_type_funder")
}
