package oracle

import "testing"

// Curse of Bounty is an Aura with "Enchant player": its bearer is a seat, not
// a permanent. Codsworth, Handy Helper's "{T}: Attach target Aura or
// Equipment you control to target creature you control" can name it, but CR
// 701.3a/b says an Aura can't be attached to a permanent it couldn't enchant
// and stays where it is, so the curse keeps enchanting its player in both
// engines. The kernel once attached it to Codsworth (decision 776 of the
// recording; pool tail-68, seed 282030).
func TestMBCCurseOfBountyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "curse_of_bounty")
}
