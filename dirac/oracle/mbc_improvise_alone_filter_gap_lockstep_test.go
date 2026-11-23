package oracle

import "testing"

// CR 702.126a with CR 601.2g-h and CR 605.3b: p0's Archway of Innovation
// emblem gives Shaper Guildmage ({U}) improvise, but no artifact credits
// anything against a cost with no generic part, so gorge plans the cast
// plainly. Its census cannot price Flooded Grove's hybrid filter ability
// ({G/U}, {T}: Add {U}{U}), so the planner is "census_incomplete" and the
// prefix search taps Slippery Karst's {G} to fund the Grove's fee: the cast
// is offered. With a granted mana ability somewhere in the table the planned
// price is withheld (FeatPlannedCast), and the kernel's gorgeLowers refused
// every contribution cast, so the any-mana walk never listed the Guildmage
// and no prefix search ran (decision 677 of tail-15-c000, pair 4).
func TestMBCShaperGuildmageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "improvise_alone_cast_filter_gap_beside_grants")
}
