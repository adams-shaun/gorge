package oracle

import "testing"

// A-Bruenor Battlehammer's static pump reads `AffectedX:Count$Valid
// Equipment.Attached/Times.2` ("+2/+0 for each Equipment attached to it"), the
// AffectedX convention: each affected creature is the reading source, so a
// creature wearing no Equipment gets nothing even while Bruenor wears some.
// The kernel once read the count off the static's own source, so every creature
// its controller had got Bruenor's bonus and the layered power differed at the
// first projection after the equipment was fastened (tail-29 pair 24: the
// creature at Cards[45] showed 6 where gorge read 4). The recording must be
// served whole so a drifted game path fails rather than replaying another game.
func TestMBCAffectedXPumpReadsAffectedCreatureLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamedServed(t, "affectedx_pump_reads_affected_creature")
}
